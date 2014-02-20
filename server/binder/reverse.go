package binder

import (
	"fmt"
	"github.com/inconshreveable/go-vhost"
	"net"
	"time"
)

func NewReverseProxyBinder(addr, publicBaseAddr string, muxTimeout time.Duration) (httpBinder *HTTPBinder, httpsBinder *HTTPBinder, err error) {
	// bind a port to listen for http connections
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return
	}

	mux, err := newReverseProxyMuxer(listener, muxTimeout)
	if err != nil {
		return
	}

	httpMux := &httpReverseProxyMuxer{VhostMuxer: mux, proto: "http"}
	httpBinder, err = sharedInit(httpMux, "http", publicBaseAddr)
	if err != nil {
		return
	}

	httpsMux := &httpReverseProxyMuxer{VhostMuxer: mux, proto: "https"}
	httpsBinder, err = sharedInit(httpsMux, "https", publicBaseAddr)
	if err != nil {
		return
	}

	return
}

// HTTPReverseProxyConn inspects the X-Forwarded-For header and
// includes either http:// or https:// in the result of Host()
type HTTPReverseProxyConn struct {
	*vhost.HTTPConn
}

func (c *HTTPReverseProxyConn) Host() string {
	proto := "http"
	if c.Request.Header.Get("X-Forwarded-Proto") == "https" {
		proto = "https"
	}
	return fmt.Sprintf("%s://%s", proto, c.HTTPConn.Host())
}

type httpReverseProxyMuxer struct {
	*vhost.VhostMuxer
	proto string
}

func (m *httpReverseProxyMuxer) Listen(hostname string) (net.Listener, error) {
	return m.VhostMuxer.Listen(fmt.Sprintf("%s://%s", m.proto, hostname))
}

func newReverseProxyMuxer(listener net.Listener, muxTimeout time.Duration) (*vhost.VhostMuxer, error) {
	fn := func(c net.Conn) (vhost.Conn, error) {
		c, err := vhost.HTTP(c)
		if err != nil {
			return nil, err
		}
		return &HTTPReverseProxyConn{c.(*vhost.HTTPConn)}, nil
	}

	return vhost.NewVhostMuxer(listener, fn, muxTimeout)
}
