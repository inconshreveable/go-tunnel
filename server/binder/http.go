package binder

import (
	"crypto/tls"
	"fmt"
	"encoding/base64"
	proto "github.com/inconshreveable/go-tunnel/proto"
	util "github.com/inconshreveable/go-tunnel/util"
	vhost "github.com/inconshreveable/go-vhost"
	"net"
	"strings"
	"time"
)

const (
	maxRandomAttempts = 10
)

func normalize(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

type vhostMuxer interface {
	Listen(string) (net.Listener, error)
	NextError() (net.Conn, error)
}

type HTTPBinder struct {
	mux            vhostMuxer       // muxer
	publicBaseAddr string           // public host or host:port address used in creating the returned URLs when binding
	proto          string           // http or https
}

func (b *HTTPBinder) Bind(rawOpts interface{}) (net.Listener, string, error) {
	var opts proto.HTTPOptions
	if err := unpackOptions(rawOpts, &opts); err != nil {
		return nil, "", err
	}

	return b.BindOpts(&opts)
}

func (b *HTTPBinder) BindOpts(opts *proto.HTTPOptions) (listener net.Listener, url string, err error) {
	for i:=0; i<maxRandomAttempts; i++ {
		// pick a name
		hostname, isRandom := b.pickName(opts)

		// bind it - this could fail if the requested hostname is already bound
		if listener, err = b.mux.Listen(hostname); err != nil {
			// only try again if we're picking names at random
			if !isRandom {
				return
			} else {
				continue
			}
		}

		// construct the public url
		url = fmt.Sprintf("%s://%s", b.proto, hostname)

		// handle http auth
		if opts.Auth != "" {
			listener = newAuthListener(listener, opts.Auth)
		}

		return
	}

	err = fmt.Errorf("Failed to assign random hostname")
	return
}

func (b *HTTPBinder) pickName(opts *proto.HTTPOptions) (url string, isRandom bool) {
	// normalize names
	hostname := normalize(opts.Hostname)
	subdomain := normalize(opts.Subdomain)

	// Register for specific hostname
	if hostname != "" {
		return hostname, false

		// Register for specific subdomain
	} else if subdomain != "" {
		return fmt.Sprintf("%s.%s", subdomain, b.publicBaseAddr), false

		// Register for random subdomain
	} else {
		return fmt.Sprintf("%s.%s", util.RandId(4), b.publicBaseAddr), true
	}
}

const (
	httpResponseTemplate = `HTTP/1.0 %s
Content-Length: %d

%s`
)

func (b *HTTPBinder) handleMuxErrors() {
	makeResponse := func(status, content string) []byte {
		return []byte(fmt.Sprintf(httpResponseTemplate, status, len(content), content))
	}

	for {
		conn, err := b.mux.NextError()

		switch err.(type) {
		case vhost.NotFound:
			msg := err.Error()
			if vconn, ok := conn.(vhost.Conn); ok {
				msg = fmt.Sprintf("Tunnel %s not found", vconn.Host())
			}
			conn.Write(makeResponse("404 Not Found", msg))
		case vhost.BadRequest:
			conn.Write(makeResponse("400 Bad Request", fmt.Sprintf("Bad request: %v", err)))
		case vhost.Closed:
			return
		default:
			if conn != nil {
				conn.Write(makeResponse("500 Internal Server Error", fmt.Sprintf("Internal Server Error: %v", err)))
			}
		}

		if conn != nil {
			conn.Close()
		}
	}
}

func sharedInit(mux vhostMuxer, protocol, publicBaseAddr string) (*HTTPBinder, error) {
	// make the binder
	binder := &HTTPBinder{
		mux:            mux,
		publicBaseAddr: normalize(publicBaseAddr),
		proto:          protocol,
	}

	// start handle muxing errors
	go binder.handleMuxErrors()

	return binder, nil
}

func NewHTTPBinder(addr, publicBaseAddr string, muxTimeout time.Duration) (*HTTPBinder, error) {
	// bind a port to listen for http connections
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	// create a new muxer that will let us bind virtual hostnames
	mux, err := vhost.NewHTTPMuxer(listener, muxTimeout)
	if err != nil {
		return nil, err
	}

	return sharedInit(mux, "http", publicBaseAddr)
}

func NewHTTPSBinder(addr, publicBaseAddr string, muxTimeout time.Duration, tlsConfig *tls.Config) (*HTTPBinder, error) {
	// bind a port to listen for https connections
	listener, err := tls.Listen("tcp", addr, tlsConfig)
	if err != nil {
		return nil, err
	}

	// create a new muxer that will let us bind virtual hostnames
	mux, err := vhost.NewHTTPMuxer(listener, muxTimeout)
	if err != nil {
		return nil, err
	}

	return sharedInit(mux, "https", publicBaseAddr)
}

type authListener struct {
	net.Listener
	encodedAuth string
}

func (a *authListener) Accept() (net.Conn, error) {
	for {
		c, err := a.Listener.Accept()
		if err != nil {
			return nil, err
		}

		httpConn, ok := c.(*vhost.HTTPConn)
		if !ok {
			return nil, fmt.Errorf("Accepted conn %v is not *vhost.HTTPConn", c)
		}

		// If the http auth doesn't match this request's auth
		// then fail the request with 401 Not Authorized and request the client reissue the
		// request with basic auth
		auth := httpConn.Request.Header.Get("Authorization")
		if auth != a.encodedAuth {
//			c.Info("Authentication failed: %s", auth)
fmt.Printf("AUTH: %v, ENCODEDAUTH: %v", auth, a.encodedAuth)
			c.Write([]byte(`HTTP/1.0 401 Not Authorized
WWW-Authenticate: Basic realm="go-tunnel"
Content-Length: 22

Authorization required`))
			c.Close()

			// accept the next connection if the auth fails
			continue
		}

		return c, nil
	}
}

func newAuthListener(l net.Listener, auth string) *authListener {
	// pre-encode the http basic auth for fast comparisons later
	return &authListener {
		Listener: l,
		encodedAuth: "Basic " + base64.StdEncoding.EncodeToString([]byte(auth)),
	}
}
