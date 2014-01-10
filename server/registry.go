package server

import (
	"sync"
)

// keeps a mapping from session id -> session
// responsible for correctly allowing a reconnecting session to safely
// replace its older instance
type sessionRegistry struct {
	sync.Mutex
	registry map[string]*Session
}

func newSessionRegistry() *sessionRegistry {
	return &sessionRegistry{
		registry: make(map[string]*Session),
	}
}

func (s *sessionRegistry) register(sess *Session) {
	s.Lock()
	old, ok := s.registry[sess.id]
	if ok {
		defer func() {
			// make sure the old session instance doesn't remove the new one
			// when it shuts down
			old.id = ""
			old.Shutdown()
		}()
	}

	s.registry[sess.id] = sess
	s.Unlock()
}

func (s *sessionRegistry) unregister(sess *Session) error {
	s.Lock()
	defer s.Unlock()
	delete(s.registry, sess.id)

	return nil
}
