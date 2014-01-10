package tls

import (
	"crypto/tls"
	"io/ioutil"
)

func ServerConfig(crtPath string, keyPath string) (tlsConfig *tls.Config, err error) {
	var crtBytes, keyBytes []byte
	if crtBytes, err = ioutil.ReadFile(crtPath); err != nil {
		return
	}
	if keyBytes, err = ioutil.ReadFile(keyPath); err != nil {
		return
	}

	return serverConfigFromBytes(crtBytes, keyBytes)
}

func ServerSnakeoil() (tlsConfig *tls.Config, err error) {
	crt, err := Asset("assets/tls/snakeoil.crt")
	if err != nil {
		return
	}

	key, err := Asset("assets/tls/snakeoil.key")
	if err != nil {
		return
	}

	tlsConfig, err = serverConfigFromBytes(crt, key)
	if err != nil {
		return
	}

	return
}

func serverConfigFromBytes(crt, key []byte) (tlsConfig *tls.Config, err error) {
	var cert tls.Certificate
	if cert, err = tls.X509KeyPair(crt, key); err != nil {
		return
	}

	tlsConfig = &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	return
}
