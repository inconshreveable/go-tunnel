package server

import (
	"fmt"
	conn "github.com/inconshreveable/go-tunnel/conn"
	log "github.com/inconshreveable/go-tunnel/log"
	proto "github.com/inconshreveable/go-tunnel/proto"
	"net"
	"runtime/debug"
	"sync/atomic"
	"time"
)

// Tunnel encapsulates a single binding of a vhost or port over a tunneling session
type Tunnel struct {
	// request that opened the tunnel
	req *proto.Bind

	// time when the tunnel was opened
	start time.Time

	// public url
	url string

	// tcp listener
	listener net.Listener

	// parent session
	sess *Session

	// logger
	log.Logger

	// closing
	closing int32

	// tunnel hooks
	hooks TunnelHooks
}

type TunnelHooks interface {
	OnConnectionOpen(*Tunnel, conn.Conn) error
	OnConnectionClose(*Tunnel, conn.Conn, time.Duration, int64, int64) error
	OnTunnelOpen(*Tunnel) error
	OnTunnelClose(*Tunnel) error
}

// Create a new tunnel from a registration message received
// on a control channel
func NewTunnel(b *proto.Bind, sess *Session, binders Binders, hooks TunnelHooks) (t *Tunnel, err error) {
	t = &Tunnel{
		req:    b,
		start:  time.Now(),
		sess:   sess,
		Logger: log.NewTaggedLogger(sess.id, "tunnel"),
		hooks:  hooks,
	}

	binder, ok := binders[t.req.Protocol]
	if !ok {
		return nil, fmt.Errorf("Can't bind for %s connections", t.req.Protocol)
	}

	if t.listener, t.url, err = binder.Bind(b.Options); err != nil {
		return
	}

	go t.listen(t.listener)

	// update the logger
	t.Logger.AddTags(t.url)

	// tunnel hook
	if err = t.hooks.OnTunnelOpen(t); err != nil {
		return
	}

	return
}

func (t *Tunnel) shutdown() error {
	t.Info("Shutting down")

	// mark that we're shutting down
	if !atomic.CompareAndSwapInt32(&t.closing, 0, 1) {
		return fmt.Errorf("Already shutting down")
	}

	// shut down the public listener
	if err := t.listener.Close(); err != nil {
		return err
	}

	// call close hook
	if err := t.hooks.OnTunnelClose(t); err != nil {
		t.Error("OnTunnelClose hook failed: %v", err)
		return err
	}

	t.Info("Shutdown complete")
	return nil
}

func (t *Tunnel) id() string {
	return t.url
}

func (t *Tunnel) recoverPanic(name string) {
	if r := recover(); r != nil {
		t.Error("%s failed with error %v: %s", name, r, debug.Stack())
	}
}

// Listens for new public connections from the internet.
func (t *Tunnel) listen(listener net.Listener) {
	defer t.recoverPanic("Tunnel.listen")

	t.Info("Listening for connections on %s", listener.Addr())
	for {
		// accept public connections
		publicConn, err := listener.Accept()

		if err != nil {
			// not an error, we're shutting down this tunnel
			if atomic.LoadInt32(&t.closing) == 1 {
				return
			}

			t.Error("Failed to accept new connection: %v", err)
			continue
		}

		go t.handlePublic(conn.Wrap(publicConn, t.url))
	}
}

func (t *Tunnel) handlePublic(publicConn conn.Conn) {
	defer publicConn.Close()
	defer t.recoverPanic("Tunnel.handlePublic")

	publicConn.Info("New connection from %v", publicConn.RemoteAddr())

	// connection hook
	if err := t.hooks.OnConnectionOpen(t, publicConn); err != nil {
		t.Error("OnConnectionOpen hook failed: %v", err)
		return
	}

	startTime := time.Now()

	// open a proxy stream
	proxyConn, err := t.sess.OpenProxy(publicConn.RemoteAddr().String(), t.url)
	if err != nil {
		t.Error("Failed to open proxy connection: %v", err)
		return
	}
	defer proxyConn.Close()

	// join the public and proxy connections
	bytesIn, bytesOut := conn.Join(publicConn, proxyConn)

	if err = t.hooks.OnConnectionClose(t, publicConn, time.Now().Sub(startTime), bytesIn, bytesOut); err != nil {
		t.Error("OnConnectionClose hook failed: %v", err)
		return
	}
}
