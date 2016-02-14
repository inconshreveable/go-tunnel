package client

import (
	"../conn"
	"../proto"
	"errors"
	"github.com/inconshreveable/muxado"
	"math"
	"sync"
	"time"
)

const (
	maxWait = 30 * time.Second
)

// This is a raw session object that has two important properties:
// 1. if a call to accept would have failed, it initiates a reconnect attempt instead
// 2. in order to facilitate the reconnect, it protects the RawSession's mux and allows it to be swapped out
type reconnectingRaw struct {
	*RawSession
	sync.RWMutex
	reconnect func() error
}

func (s *reconnectingRaw) Listen(protocol string, opts interface{}, extra interface{}) (resp *proto.BindResp, err error) {
	s.RLock()
	defer s.RUnlock()
	return s.RawSession.Listen(protocol, opts, extra)
}

func (s *reconnectingRaw) Unlisten(url string) (resp *proto.UnbindResp, err error) {
	s.RLock()
	defer s.RUnlock()
	return s.RawSession.Unlisten(url)
}

func (s *reconnectingRaw) Accept() (conn.Conn, error) {
	for {

		s.RLock()
		c, err := s.RawSession.Accept()
		s.RUnlock()

		// if we get an error, reconnect instead of returning it
		if err != nil {
			s.Error("Error from Accept(): %v, reconnecting . . .", err)
			// reconnect can still return errors for permanent failures
			if err = s.reconnect(); err != nil {
				return nil, err
			}
		} else {
			return c, err
		}
	}
}

type ReconnectingSession struct {
	dialer    func() (muxado.Session, error)
	authExtra interface{}
	done      chan error
	raw       *reconnectingRaw
	*Session
}

func NewReconnectingSession(dialer func() (muxado.Session, error), authExtra interface{}) (*ReconnectingSession, error) {
	s := &ReconnectingSession{
		dialer:    dialer,
		done:      make(chan error, 1),
		authExtra: authExtra,
		Session: &Session{
			tunnels: make(map[string]*Tunnel),
		},
		raw: &reconnectingRaw{RawSession: NewRawSession(nil)},
	}

	// wire up the circular references
	s.raw.reconnect = s.reconnect
	s.Session.raw = s.raw

	// setup an initial connection before we return
	if err := s.reconnect(); err != nil {
		return nil, err
	}

	go s.Session.receive()

	return s, nil
}

func (s *ReconnectingSession) reconnect() error {
	var wait time.Duration = time.Second

	failTemp := func(err error) {
		s.raw.Info("Session failed: %v", err)

		// session failed, wait before reconnecting
		s.raw.Info("Waiting %d seconds before reconnecting", int(wait.Seconds()))
		time.Sleep(wait)

		// exponentially increase wait time up to a limit
		wait = 2 * wait
		wait = time.Duration(math.Min(float64(wait), float64(maxWait)))
	}

	fail := func(err error) error {
		s.done <- err
		return err
	}

retry:
	// dial the tunnel server
	mux, err := s.dialer()
	if err != nil {
		failTemp(err)
		goto retry
	}

	// swap the muxado session in
	s.raw.Lock()
	s.raw.mux = mux
	s.raw.Unlock()

	resp, err := s.raw.Auth(s.raw.id, s.authExtra)
	if err != nil {
		failTemp(err)
		goto retry
	}

	// auth errors are considered permanent
	if resp.Error != "" {
		return fail(errors.New(resp.Error))
	}

	// re-establish binds
	s.RLock()
	for _, t := range s.tunnels {
		resp, err := s.raw.Listen(t.proto, t.bindOpts, t.bindExtra)
		if err != nil {
			s.RUnlock()
			failTemp(err)
			goto retry
		}

		if resp.Error != "" {
			s.RUnlock()
			return fail(errors.New(resp.Error))
		}
	}
	s.RUnlock()

	return nil
}

func (s *ReconnectingSession) Wait() error {
	return <-s.done
}
