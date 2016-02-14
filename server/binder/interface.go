package binder

import (
	"../util"
	"fmt"
	"net"
)

func pickName(hostname, subdomain, publicBaseAddr string) (url string, israndom bool) {
	// normalize names
	hostname = normalize(hostname)
	subdomain = normalize(subdomain)

	// register for specific hostname
	if hostname != "" {
		return hostname, false

		// register for specific subdomain
	} else if subdomain != "" {
		return fmt.Sprintf("%s.%s", subdomain, publicBaseAddr), false

		// register for random subdomain
	} else {
		return fmt.Sprintf("%s.%s", util.RandId(4), publicBaseAddr), true
	}
}

type Binder interface {
	Bind(interface{}) (net.Listener, string, error)
}
