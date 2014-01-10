package server

import (
	"crypto/tls"
	log "github.com/inconshreveable/go-tunnel/log"
	"github.com/inconshreveable/go-tunnel/server/binder"
	muxado "github.com/inconshreveable/muxado"
	"net"
)

type Binders map[string]binder.Binder

type Server struct {
	log.Logger                  // logger for the server object
	listener   *muxado.Listener // listener for new sessions
	registry   *sessionRegistry
	Binders
	SessionHooks
	TunnelHooks
}

func Serve(network, addr string, binders Binders) (*Server, error) {
	l, err := net.Listen(network, addr)
	if err != nil {
		return nil, err
	}

	return NewServer(l, binders), nil
}

func ServeTLS(network, addr string, tlsConfig *tls.Config, binders Binders) (*Server, error) {
	l, err := tls.Listen(network, addr, tlsConfig)
	if err != nil {
		return nil, err
	}

	return NewServer(l, binders), nil
}

func NewServer(listener net.Listener, binders Binders) *Server {
	return &Server{
		Logger:       log.NewTaggedLogger("server", listener.Addr().String()),
		listener:     muxado.NewListener(listener),
		registry:     newSessionRegistry(),
		Binders:      binders,
		TunnelHooks:  new(NoopTunnelHooks),
		SessionHooks: new(NoopSessionHooks),
	}
}

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
