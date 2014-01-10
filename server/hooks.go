package server

import (
	conn "github.com/inconshreveable/go-tunnel/conn"
	proto "github.com/inconshreveable/go-tunnel/proto"
	"time"
)

type NoopSessionHooks int

func (*NoopSessionHooks) OnAuth(*Session, *proto.Auth) error { return nil }
func (*NoopSessionHooks) OnBind(*Session, *proto.Bind) error { return nil }
func (*NoopSessionHooks) OnClose(*Session) error             { return nil }

type NoopTunnelHooks int

func (*NoopTunnelHooks) OnConnectionOpen(*Tunnel, conn.Conn) error { return nil }
func (*NoopTunnelHooks) OnConnectionClose(*Tunnel, conn.Conn, time.Duration, int64, int64) error {
	return nil
}
func (*NoopTunnelHooks) OnTunnelOpen(*Tunnel) error  { return nil }
func (*NoopTunnelHooks) OnTunnelClose(*Tunnel) error { return nil }
