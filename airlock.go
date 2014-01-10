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

// defaultClient returns a new client session connected to the default
// public tunneling service hosted at airlock.io
func defaultClient(authtoken string) (*client.Session, error) {
	trusted, err := tuntls.ClientTrusted(defaultSNI)
	if err != nil {
		return nil, err
	}

	sess, err := DialTLS("tcp", defaultAddr, trusted)
	if err != nil {
		return nil, err
	}

	if err = sess.Auth(authtoken, nil); err != nil {
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

	return c.BindHTTP(new(proto.HTTPOptions), nil)
}

// ListenHTTPS begins listening for HTTPS connections on a randomly-assigned
// hostname of the public tunneling service at airlock.io
/*
func ListenHTTPS() (*client.Tunnel, error) {
	c, err := defaultClient("")
	if err != nil {
		return nil, err
	}

	return c.BindHTTPS(new(proto.HTTPOptions), nil)
}
*/

// ListenTCP begins listening for TCP connections on a randomly-assigned
// port of the public tunneling service at airlock.io
func ListenTCP() (*client.Tunnel, error) {
	c, err := defaultClient("")
	if err != nil {
		return nil, err
	}

	return c.BindTCP(new(proto.TCPOptions), nil)
}
