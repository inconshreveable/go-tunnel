package binder

import (
	"fmt"
	proto "github.com/inconshreveable/go-tunnel/proto"
	vhost "github.com/inconshreveable/go-vhost"
	"net"
	"time"
)

type TLSBinder struct {
	mux            vhostMuxer // muxer
	publicBaseAddr string     // public host or host:port address used in creating the returned URLs when binding
}

func NewTLSBinder(addr, publicBaseAddr string, muxTimeout time.Duration) (*TLSBinder, error) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	mux, err := vhost.NewTLSMuxer(listener, muxTimeout)
	if err != nil {
		return nil, err
	}

	binder := &TLSBinder{
		mux:            mux,
		publicBaseAddr: publicBaseAddr,
	}

	go mux.HandleErrors()

	return binder, nil
}

func (b *TLSBinder) Bind(rawOpts interface{}) (net.Listener, string, error) {
	var opts proto.TLSOptions
	if err := unpackOptions(rawOpts, &opts); err != nil {
		return nil, "", err
	}

	return b.BindOpts(&opts)
}

func (b *TLSBinder) BindOpts(opts *proto.TLSOptions) (listener net.Listener, url string, err error) {
	for i := 0; i < maxRandomAttempts; i++ {
		// pick a name
		hostname, isRandom := pickName(opts.Hostname, opts.Subdomain, b.publicBaseAddr)

		// bind it - this could fail if the requested hostname is already bound
		if listener, err = b.mux.Listen(hostname); err != nil {
			// only try again if we're picking names at random
			if !isRandom {
				return
			} else {
				continue
			}
		}

		// construct the public url
		url = fmt.Sprintf("tls://%s", hostname)

		return
	}

	err = fmt.Errorf("Failed to assign random hostname")
	return
}
