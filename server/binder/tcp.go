package binder

import (
	"fmt"
	"github.com/inconshreveable/go-tunnel/proto"
	"net"
	"strings"
)

type TCPBinder struct {
	iface    net.IP // the interface to bind TCP ports on
	hostname string // a public hostname of the address where the ports are bound
}

func (b *TCPBinder) Bind(rawOpts interface{}) (net.Listener, string, error) {
	var opts proto.TCPOptions
	if err := unpackOptions(rawOpts, &opts); err != nil {
		return nil, "", err
	}

	return b.BindOpts(&opts)
}

func (b *TCPBinder) BindOpts(opts *proto.TCPOptions) (listener net.Listener, url string, err error) {
	// create the listening address
	listenAddr := &net.TCPAddr{
		IP:   b.iface,
		Port: int(opts.RemotePort),
	}

	// bind a new tcp port
	if listener, err = net.ListenTCP("tcp", listenAddr); err != nil {
		return
	}

	// we ask the listener what port it bound in case
	// the client supplied port 0 and the OS picked one at random
	addr := listener.Addr().(*net.TCPAddr)
	url = fmt.Sprintf("tcp://%s:%d", b.hostname, addr.Port)
	return
}

// Create a new TCP binder that binds ports on the given interface.
// The supplied hostname is only used for "display" purposes to
// communicate back to the clients the public hostname where
// the bound port can be accessed.
func NewTCPBinder(iface string, hostname string) *TCPBinder {
	return &TCPBinder{
		iface:    net.ParseIP(iface),
		hostname: strings.ToLower(hostname),
	}
}
