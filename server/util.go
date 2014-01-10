package server

import (
	"errors"
	"sync"
)

// facilitates controlled shutdowns of resources
type shutdownGuard struct {
	l        sync.RWMutex
	shutting bool
}

func (g *shutdownGuard) Enter() error {
	g.l.RLock()
	if g.shutting {
		g.l.RUnlock()
		return errors.New("Shutdown in progress")
	}
	return nil
}

func (g *shutdownGuard) Exit() {
	g.l.RUnlock()
}

func (g *shutdownGuard) BeginShutdown() {
	g.l.Lock()
	g.shutting = true
}

func (g *shutdownGuard) CompleteShutdown() {
	g.l.Unlock()
}
