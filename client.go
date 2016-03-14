package tunnel

import (
	"crypto/tls"
	"net"

	client "github.com/inconshreveable/go-tunnel/client"
	"github.com/inconshreveable/muxado"
)

// Client starts a new go-tunnel session on conn
func Client(conn net.Conn) *client.Session {
	return client.NewSession(muxado.Client(conn))
}

// Dial starts a go-tunnel session on a new connection to addr
func Dial(network, addr string) (*client.Session, error) {
	mux, err := muxado.Dial(network, addr)
	if err != nil {
		return nil, err
	}
	return client.NewSession(mux), nil
}

// DialExtra starts a go-tunnel session on a new tls connection to addr
func DialTLS(network, addr string, tlsConfig *tls.Config) (*client.Session, error) {
	mux, err := muxado.DialTLS(network, addr, tlsConfig)
	if err != nil {
		return nil, err
	}
	return client.NewSession(mux), nil
}

// DialTLSReconnecting starts a go-tunnel session managed by a ReconnectingSession object
// over a new connection to addr. The ReconnectingSessions will initially Auth to the server with authExtra
// as well as whenever it reconnects.
func DialReconnecting(network, addr string, authExtra interface{}) (*client.ReconnectingSession, error) {
	dialer := func() (muxado.Session, error) {
		return muxado.Dial(network, addr)
	}

	return client.NewReconnectingSession(dialer, authExtra)
}

// DialTLSReconnecting starts a go-tunnel session managed by a ReconnectingSession object
// over a new TLS connection to addr. The ReconnectingSessions will initially Auth to the server with authExtra
// as well as whenever it reconnects.
func DialTLSReconnecting(network, addr string, tlsConfig *tls.Config, authExtra interface{}) (*client.ReconnectingSession, error) {
	dialer := func() (muxado.Session, error) {
		return muxado.DialTLS(network, addr, tlsConfig)
	}

	return client.NewReconnectingSession(dialer, authExtra)
}
