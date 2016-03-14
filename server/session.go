package server

import (
	conn "../conn"
	log "../log"
	proto "../proto"
	util "../util"
	"fmt"
	muxado "github.com/inconshreveable/muxado"
	"reflect"
	"runtime/debug"
	"sort"
	"sync"
	"time"
)

type Session struct {
	// logger
	log.Logger

	// auth message
	auth *proto.Auth

	// session start time
	start time.Time

	// underlying mux session
	mux muxado.Session

	// all of the tunnels this session handles
	tunnels map[string]*Tunnel

	// identifier
	id string

	// session hooks
	hooks SessionHooks

	// binders
	binders Binders

	// synchronization for accessing Session.tunnels
	sync.Mutex

	// hooks for tunnels
	tunnelHooks TunnelHooks

	// safe-shutdown synchronization
	guard shutdownGuard

	// registry
	registry *sessionRegistry
}

type SessionHooks interface {
	OnAuth(*Session, *proto.Auth) error
	OnBind(*Session, *proto.Bind) error
	OnClose(*Session) error
}

func NewSession(mux muxado.Session, registry *sessionRegistry, sessHooks SessionHooks, tunnelHooks TunnelHooks, binders Binders) *Session {
	return &Session{
		start:       time.Now(),
		Logger:      log.NewTaggedLogger("session"),
		mux:         mux,
		tunnels:     make(map[string]*Tunnel, 0),
		registry:    registry,
		hooks:       sessHooks,
		binders:     binders,
		tunnelHooks: tunnelHooks,
	}
}

// Run runs the the tunnel session
func (s *Session) Run() (err error) {
	defer s.recoverPanic("Session.Run")

	go func() {
		defer s.recoverPanic("Session.mux.Wait")
		code, err, debug := s.mux.Wait()
		s.Info("Session mux shutdown with code %v error %v debug %v", code, err, debug)
	}()

	defer s.mux.Close()

	// A tunnel session starts with an auth stream
	if err = s.handleAuth(); err != nil {
		return
	}

	// then we handle new streams sent from the client
	for {
		stream, err := s.mux.Accept()
		if err != nil {
			s.Shutdown()
			return s.Error("Failed to accept stream: %v", err)
		}

		go s.handleStream(conn.Wrap(stream, "stream", s.id))
	}
}

func (s *Session) handleAuth() error {
	// accept ann auth stream
	raw, err := s.mux.Accept()
	if err != nil {
		return s.Error("Failed to accept auth stream: %v", err)
	}
	defer raw.Close()

	stream := conn.Wrap(raw, "session", "auth")

	// read the Auth message
	if err = proto.ReadMsgInto(stream, &s.auth); err != nil {
		return s.Error("Failed to read auth message; %v", err)
	}

	failAuth := func(e error) error {
		_ = proto.WriteMsg(stream, &proto.AuthResp{Error: e.Error()})
		return e
	}

	// generate a client identifier
	s.id = s.auth.ClientId
	if s.id == "" {
		// it's a new session, assign an ID
		if s.id, err = util.SecureRandId(16); err != nil {
			return failAuth(fmt.Errorf("Failed generate client identifier: %v", err))
		}
	}

	// put ourselves in the registry
	s.registry.register(s)

	// set logging prefix
	s.Logger.AddTags(s.id)

	// agree on protocol version
	// if proto.Version not in s.auth.Version
	if sort.SearchStrings(s.auth.Version, proto.Version) == len(s.auth.Version) {
		return failAuth(fmt.Errorf("No acceptable protocol version. Requested: %v, capable: %v", s.auth.Version, proto.Version))
	}

	// auth hook
	if err = s.hooks.OnAuth(s, s.auth); err != nil {
		return failAuth(err)
	}

	// Respond to authentication
	authResp := &proto.AuthResp{
		Version:  proto.Version,
		ClientId: s.id,
	}

	if err = proto.WriteMsg(stream, authResp); err != nil {
		return failAuth(fmt.Errorf("Failed to write authentication response: %v", err))
	}

	return nil
}

func (s *Session) handleStream(stream conn.Conn) {
	defer s.recoverPanic("Session.handleStream")
	defer stream.Close()

	// make sure we only process streams while we're not shutting down
	if err := s.guard.Enter(); err != nil {
		stream.Error("Failing stream, session is shutting down")
		return
	}
	defer s.guard.Exit()

	raw, err := proto.ReadMsg(stream)
	if err != nil {
		stream.Error("Failed to read message: %v")
		go s.Shutdown()
		return
	}

	switch msg := raw.(type) {
	case *proto.Bind:
		err = s.handleBind(stream, msg)
	case *proto.Unbind:
		err = s.handleUnbind(stream, msg)
	default:
		err = fmt.Errorf("Unknown message type: %v", reflect.TypeOf(raw))
	}

	if err != nil {
		stream.Error("Error on stream: %v", err)
		go s.Shutdown()
		return
	}

	return
}

func (s *Session) handleBind(stream conn.Conn, bind *proto.Bind) (err error) {
	stream.Debug("Binding new tunnel: %v", bind)

	respond := func(resp *proto.BindResp) {
		if err = proto.WriteMsg(stream, resp); err != nil {
			err = stream.Error("Failed to send bind response: %v", err)
		}
	}

	if err = s.hooks.OnBind(s, bind); err != nil {
		return
	}

	t, err := newTunnel(bind, s, s.binders, s.tunnelHooks)
	if err != nil {
		respond(&proto.BindResp{Error: err.Error()})
		return
	}
	t.Info("Registered new tunnel on session %s", s.id)

	// add it to the list of tunnels
	s.addTunnel(t)

	// acknowledge success
	respond(&proto.BindResp{Url: t.url})
	return
}

func (s *Session) handleUnbind(stream conn.Conn, unbind *proto.Unbind) (err error) {
	s.Debug("Unbinding tunnel")

	// remote it from the list of tunnels
	t, ok := s.delTunnel(unbind.Url)
	if !ok {
		return s.Error("Failed to unbind tunnel %s: no tunnel found.", unbind.Url)
	}

	if err = t.shutdown(); err != nil {
		return s.Error("Failed to unbind tunnel %s: %v", unbind.Url, err)
	}

	// acknowledge success
	unbindResp := &proto.UnbindResp{}
	if err = proto.WriteMsg(stream, unbindResp); err != nil {
		return s.Error("Failed to write unbind resp: %v", err)
	}

	return
}

func (s *Session) Shutdown() {
	s.recoverPanic("Session.Shutdown")

	s.guard.BeginShutdown()
	defer s.guard.CompleteShutdown()

	// run shutdown hooks
	if err := s.hooks.OnClose(s); err != nil {
		s.Error("OnClose hook failed with error: %v", err)
		return
	}

	// shutdown all of the tunnels
	for _, t := range s.tunnels {
		t.shutdown()
	}

	// remove ourselves from the registry
	s.registry.unregister(s)

	// close underlying mux session
	s.mux.Close()

	s.Info("Shutdown complete")
}

// Opens a new proxy stream to the client and writes a StartProxy message
// with the given client address and tunnel url.
func (s *Session) openProxy(clientAddr, tunnelUrl string) (pxy conn.Conn, err error) {
	// open a new proxy stream
	pxyStream, err := s.mux.Open()
	if err != nil {
		return
	}
	pxy = conn.Wrap(pxyStream)

	// tell the client we're going to start using this proxy connection
	startProxy := &proto.StartProxy{
		ClientAddr: clientAddr,
		Url:        tunnelUrl,
	}

	if err = proto.WriteMsg(pxy, startProxy); err != nil {
		return
	}

	pxy.AddTags(tunnelUrl)
	return
}

func (s *Session) addTunnel(t *Tunnel) {
	s.Lock()
	defer s.Unlock()
	s.tunnels[t.url] = t
}

func (s *Session) delTunnel(url string) (*Tunnel, bool) {
	s.Lock()
	defer s.Unlock()
	t, ok := s.tunnels[url]
	if ok {
		delete(s.tunnels, url)
	}
	return t, ok
}

func (s *Session) recoverPanic(name string) {
	if r := recover(); r != nil {
		s.Error("%s failed with error %v: %s", name, r, debug.Stack())
	}
}

func (s *Session) Auth() *proto.Auth {
	return s.auth
}

func (s *Session) Id() string {
	return s.id
}

func (s *Session) Start() time.Time {
	return s.start
}
