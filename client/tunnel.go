package client

import (
	conn "../conn"
	proto "../proto"
	"errors"
	"fmt"
	"net"
	"sync/atomic"
)

// A Tunnel is a net.Listener that Accept()'s connections from a
// remote machine.
type Tunnel struct {
	url       string
	sess      *Session
	bindOpts  interface{}
	bindExtra interface{}
	bindResp  *proto.BindResp
	accept    chan conn.Conn
	proto     string
	closed    int32
}

func (t *Tunnel) Accept() (net.Conn, error) {
	select {
	case conn, ok := <-t.accept:
		if !ok {
			return nil, errors.New("Tunnel closed")
		}
		return conn, nil
	}
	return nil, errors.New("Unexpected select condition")
}

func (t *Tunnel) Close() error {
	if !atomic.CompareAndSwapInt32(&t.closed, 0, 1) {
		return fmt.Errorf("Already closed")
	}

	close(t.accept)
	return t.sess.unlisten(t)
}

func (t *Tunnel) Addr() net.Addr {
	return &Addr{net: t.proto, addr: t.url}
}

type Addr struct {
	net  string
	addr string
}

func (a *Addr) Network() string {
	return a.net
}

func (a *Addr) String() string {
	return a.addr
}
