package tunnel

import (
	client "github.com/inconshreveable/go-tunnel/client"
	proto "github.com/inconshreveable/go-tunnel/proto"
	tuntls "github.com/inconshreveable/go-tunnel/tls"
)

const (
	defaultAddr = "v1.airlock.io:443"
	defaultSNI  = "v1.airlock.io"
)

type airlockAuthExtra struct {
	Token string
}

// defaultClient returns a new client session connected to the default
// public tunneling service hosted at airlock.io
func defaultClient(authtoken string) (*client.ReconnectingSession, error) {
	trusted, err := tuntls.ClientTrusted(defaultSNI)
	if err != nil {
		return nil, err
	}

	authExtra := airlockAuthExtra{Token: authtoken}
	sess, err := DialTLSReconnecting("tcp", defaultAddr, trusted, authExtra)
	if err != nil {
		return nil, err
	}

	return sess, nil
}

// ListenHTTP begins listening for HTTP connections on a randomly-assigned
// hostname of the public tunneling service at airlock.io
func ListenHTTP() (*client.Tunnel, error) {
	c, err := defaultClient("")
	if err != nil {
		return nil, err
	}

	return c.ListenHTTP(new(proto.HTTPOptions), nil)
}

// ListenHTTPS begins listening for HTTPS connections on a randomly-assigned
// hostname of the public tunneling service at airlock.io
/*
func ListenHTTPS() (*client.Tunnel, error) {
	c, err := defaultClient("")
	if err != nil {
		return nil, err
	}

	return c.ListenHTTPS(new(proto.HTTPOptions), nil)
}
*/

// ListenTCP begins listening for TCP connections on a randomly-assigned
// port of the public tunneling service at airlock.io
func ListenTCP() (*client.Tunnel, error) {
	c, err := defaultClient("")
	if err != nil {
		return nil, err
	}

	return c.ListenTCP(new(proto.TCPOptions), nil)
}
