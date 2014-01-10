package server

import (
	"crypto/tls"
	log "github.com/inconshreveable/go-tunnel/log"
	"github.com/inconshreveable/go-tunnel/server/binder"
	muxado "github.com/inconshreveable/muxado"
	"net"
)

type Binders map[string]binder.Binder

// A Server accepts new go-tunnel connections from clients and establishes
// a Session on which it wil services their requests to listen for
// connections on the Server's ports and/or hostnames of well-known protocols.
//
// Servers with custom behaviors maybe implemented by setting the SessionHooks
// and TunnelHooks properties before calling .Run()
type Server struct {
	log.Logger                    // logger for the server object
	listener     *muxado.Listener // listener for new sessions
	registry     *sessionRegistry // map of session id -> Session
	Binders                       // a map of protocol name -> tunnel binder
	SessionHooks                  // user-defined hooks to customize session behavior
	TunnelHooks                   // user-definied hooks to customize tunnel behavior
}

// Serve creates a Server listening for new connections on the given address.
// It binds tunnels on those sessions with the given binders.
func Serve(network, addr string, binders Binders) (*Server, error) {
	l, err := net.Listen(network, addr)
	if err != nil {
		return nil, err
	}

	return NewServer(l, binders), nil
}

// ServeTLS creates a Server listening for new TLS connections which binds tunnels
// with the given binders.
func ServeTLS(network, addr string, tlsConfig *tls.Config, binders Binders) (*Server, error) {
	l, err := tls.Listen(network, addr, tlsConfig)
	if err != nil {
		return nil, err
	}

	return NewServer(l, binders), nil
}

// NewServer creates a new server which binds tunnels with binders on sessions
// it accepts from the given listener.
func NewServer(listener net.Listener, binders Binders) *Server {
	return &Server{
		Logger:       log.NewTaggedLogger("server", listener.Addr().String()),
		listener:     muxado.NewListener(listener),
		registry:     NewSessionRegistry(),
		Binders:      binders,
		TunnelHooks:  new(NoopTunnelHooks),
		SessionHooks: new(NoopSessionHooks),
	}
}

// Run loops forever accepting new tunnel sessions from remote clients
func (s *Server) Run() error {
	s.Info("Listening for tunnel sessions on %s", s.listener.Addr().String())

	for {
		sess, err := s.listener.Accept()
		if err != nil {
			s.Error("Failed to accept new tunnel session: %v", err)
			continue
		}
		s.Info("New tunnel session from: %v", sess.RemoteAddr())

		go NewSession(sess, s.registry, s.SessionHooks, s.TunnelHooks, s.Binders).Run()
	}
}
