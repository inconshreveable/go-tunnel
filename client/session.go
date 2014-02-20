package client

import (
	"errors"
	conn "github.com/inconshreveable/go-tunnel/conn"
	log "github.com/inconshreveable/go-tunnel/log"
	proto "github.com/inconshreveable/go-tunnel/proto"
	muxado "github.com/inconshreveable/muxado"
	"net"
	"strconv"
	"strings"
	"sync"
)

type rawSession interface {
	Auth(string, interface{}) (*proto.AuthResp, error)
	Listen(string, interface{}, interface{}) (*proto.BindResp, error)
	Unlisten(string) (*proto.UnbindResp, error)
	Accept() (conn.Conn, error)
	log.Logger
}

// A RawSession is a client session which handles authorization with the tunnel server, then
// listening and unlistening of tunnels.
//
// When RawSession.Accept() returns an error, that means the session is dead.
// Client sessions run over a muxado session.
type RawSession struct {
	mux        muxado.Session // the muxado session we're multiplexing streams over
	log.Logger                // logger for this client
	id         string         // session id, allows for resuming existing sessions
}

// Creates a new client tunnel session with the given id
// running over the given muxado session.
func NewRawSession(mux muxado.Session) *RawSession {
	sess := &RawSession{
		mux:    mux,
		Logger: log.NewTaggedLogger("session"),
	}

	return sess
}

// Auth sends an authentication message to the server and returns the server's response.
// The id string will be empty unless reconnecting an existing session.
// extra is an opaque struct useful for passing application-specific data.
func (s *RawSession) Auth(id string, extra interface{}) (resp *proto.AuthResp, err error) {
	req := &proto.Auth{
		ClientId: id,
		Extra:    extra,
		Version:  []string{proto.Version},
	}
	resp = new(proto.AuthResp)
	if err = s.req("auth", req, resp); err != nil {
		return
	}

	// set client id / log tag only if it changed
	if s.id != resp.ClientId {
		s.id = resp.ClientId
		s.Logger.AddTags(s.id)
	}
	return
}

// Listen sends a listen message to the server and returns the server's response
// protocol is the requested protocol to listen.
// opts are protocol-specific options for listening.
// extra is an opaque struct useful for passing application-specific data.
func (s *RawSession) Listen(protocol string, opts interface{}, extra interface{}) (resp *proto.BindResp, err error) {
	req := &proto.Bind{
		Protocol: protocol,
		Options:  opts,
		Extra:    extra,
	}
	resp = new(proto.BindResp)
	err = s.req("listen", req, resp)
	return
}

// Unlisten sends an unlisten message to the server and returns the server's response.
// url is the url of the open, bound tunnel to unlisten
func (s *RawSession) Unlisten(url string) (resp *proto.UnbindResp, err error) {
	req := &proto.Unbind{Url: url}
	resp = new(proto.UnbindResp)
	err = s.req("unlisten", req, resp)
	return
}

// Accept returns the next stream initiated by the server over the underlying muxado session
func (s *RawSession) Accept() (conn.Conn, error) {
	raw, err := s.mux.Accept()
	if err != nil {
		return nil, err
	}

	return conn.Wrap(raw, "proxy", s.id), nil
}

func (s *RawSession) req(tag string, req interface{}, resp interface{}) (err error) {
	stream, err := s.mux.Open()
	if err != nil {
		return
	}
	defer stream.Close()

	// log what happens on the stream
	c := conn.Wrap(stream, tag, s.id)

	// send the unlisten request
	if err = proto.WriteMsg(c, req); err != nil {
		return
	}

	// read out the unlisten response
	if err = proto.ReadMsgInto(c, resp); err != nil {
		return
	}

	return
}

// Session is a higher-level client session interface. You will almost always prefer this over
// RawSession.
//
// Unlike RawSession, when you listen a new tunnel on Session, you are returned a Tunnel
// object which allows you to recieve new connections from that listen.
type Session struct {
	raw rawSession
	sync.RWMutex
	tunnels map[string]*Tunnel
}

func NewSession(mux muxado.Session) *Session {
	s := &Session{
		raw:     NewRawSession(mux),
		tunnels: make(map[string]*Tunnel),
	}

	go s.receive()
	return s
}

func (s *Session) Auth(id string, extra interface{}) error {
	resp, err := s.raw.Auth(id, extra)
	if err != nil {
		return err
	}

	if resp.Error != "" {
		return errors.New(resp.Error)
	}

	return nil
}

// Listen negotiates with the server to create a new remote listen for the given protocol
// and options. It returns a *Tunnel on success from which the caller can accept new
// connections over the listen.
//
// Applications will typically prefer to call the protocol-specific methods like
// ListenHTTP, ListenTCP, etc.
func (s *Session) Listen(protocol string, opts interface{}, extra interface{}) (*Tunnel, error) {
	resp, err := s.raw.Listen(protocol, opts, extra)
	if err != nil {
		return nil, err
	}

	// process application-level error
	if resp.Error != "" {
		return nil, errors.New(resp.Error)
	}

	// if you asked for a random domain or random port, remember the value
	// the server assigned you for reconnection cases
	switch o := opts.(type) {
	case *proto.HTTPOptions:
		if o.Subdomain == "" && o.Hostname == "" {
			o.Hostname = strings.Split(resp.Url, "://")[1]
		}
	case *proto.TCPOptions:
		if o.RemotePort == 0 {
			parts := strings.Split(resp.Url, ":")
            portString := parts[len(parts)-1]
			port, err := strconv.ParseUint(portString, 10, 16)
			if err != nil {
				return nil, err
			}
			o.RemotePort = uint16(port)
		}
	}

	// make tunnel
	t := &Tunnel{
		url:       resp.Url,
		bindOpts:  opts,
		bindExtra: extra,
		bindResp:  resp,
		sess:      s,
		accept:    make(chan conn.Conn),
		proto:     protocol,
	}

	// add to tunnel registry
	s.addTunnel(resp.Url, t)

	return t, nil
}

// ListenHTTP listens on a new HTTP endpoint and returns a *Tunnel which accepts connections on the remote listener.
func (s *Session) ListenHTTP(opts *proto.HTTPOptions, extra interface{}) (*Tunnel, error) {
	return s.Listen("http", opts, extra)
}

// ListenHTTP listens on a new HTTPS endpoint and returns a *Tunnel which accepts connections on the remote listener.
func (s *Session) ListenHTTPS(opts *proto.HTTPOptions, extra interface{}) (*Tunnel, error) {
	return s.Listen("https", opts, extra)
}

// ListenHTTPAndHTTPS listens a new HTTP and HTTPS endpoint on the same hostname. It returns a two *Tunnel objects which accept connections on the remote HTTP and HTTPS listens, respectively.
func (s *Session) ListenHTTPAndHTTPS(opts *proto.HTTPOptions, extra interface{}) (*Tunnel, *Tunnel, error) {
	t1, err := s.Listen("http", opts, extra)
	if err != nil {
		return nil, nil, err
	}

	// the first Listen call for "http" will transform opts to be deterministic if the caller
	// asked for random
	t2, err := s.Listen("https", opts, extra)
	if err != nil {
		t1.Close()
		return nil, nil, err
	}

	return t1, t2, nil
}

// ListenTLS listens on a new TCP endpoint and returns a *Tunnel which accepts connections on the remote listener.
func (s *Session) ListenTCP(opts *proto.TCPOptions, extra interface{}) (*Tunnel, error) {
	return s.Listen("tcp", opts, extra)
}

// ListenTLS listens on a new TLS endpoint and returns a *Tunnel which accepts connections on the remote listener.
func (s *Session) ListenTLS(opts *proto.TLSOptions, extra interface{}) (*Tunnel, error) {
    return s.Listen("tls", opts, extra)
}

func (s *Session) receive() {
	handleProxy := func(proxy conn.Conn) {
		// read out the proxy message
		var startPxy proto.StartProxy
		if err := proto.ReadMsgInto(proxy, &startPxy); err != nil {
			proxy.Error("Server failed to write StartProxy: %v", err)
			proxy.Close()
			return
		}

		// wrap connection so that it has a proper RemoteAddr()
		proxy = &proxyConn{Conn: proxy, remoteAddr: &proxyAddr{startPxy.ClientAddr}}

		// find tunnel
		tunnel, ok := s.getTunnel(startPxy.Url)
		if !ok {
			proxy.Error("Couldn't find tunnel for proxy: %s", startPxy.Url)
			proxy.Close()
			return
		}

		// deliver proxy connection
		tunnel.accept <- proxy
	}

	for {
		// accept the next proxy connection
		proxy, err := s.raw.Accept()
		if err != nil {
			s.raw.Error("Client accept error: %v", err)
			s.RLock()
			for _, t := range s.tunnels {
				go t.Close()
			}
			s.RUnlock()
			return
		}
		go handleProxy(proxy)
	}
}

func (s *Session) unlisten(t *Tunnel) error {
	// delete tunnel
	s.delTunnel(t.url)

	// ask server to unlisten
	resp, err := s.raw.Unlisten(t.url)
	if err != nil {
		return err
	}

	if resp.Error != "" {
		return s.raw.Error("Server failed to unlisten tunnel: %v", resp.Error)
	}

	return nil
}

func (s *Session) getTunnel(url string) (t *Tunnel, ok bool) {
	s.RLock()
	defer s.RUnlock()
	t, ok = s.tunnels[url]
	return
}

func (s *Session) addTunnel(url string, t *Tunnel) {
	s.Lock()
	defer s.Unlock()
	s.tunnels[url] = t
}

func (s *Session) delTunnel(url string) {
	s.Lock()
	defer s.Unlock()
	delete(s.tunnels, url)
}

type proxyConn struct {
	conn.Conn
	remoteAddr net.Addr
}

func (c *proxyConn) RemoteAddr() net.Addr {
	return c.remoteAddr
}

type proxyAddr struct {
	addr string
}

func (a *proxyAddr) String() string {
	return a.addr
}

func (a *proxyAddr) Network() string {
	return "tcp"
}
