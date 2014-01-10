package tls

import (
	"crypto/tls"
	"io/ioutil"
	"crypto/x509"
	"encoding/pem"
	"fmt"
)

func ClientConfig(servername string, rootPaths []string) (*tls.Config, error) {
	roots := make([][]byte, 0)

	for _, certPath := range rootPaths {
		bytes, err := ioutil.ReadFile(certPath)
		if err != nil {
			return nil, err
		}
		roots = append(roots, bytes)
	}

	return clientConfigFromBytes(servername, roots)
}

func ClientSnakeoil() (tlsConfig *tls.Config, err error) {
	root, err := Asset("assets/tls/snakeoil.root.crt")
	if err != nil {
		return
	}

	return clientConfigFromBytes("snakeoil.example.com", [][]byte{root})
}

func ClientTrusted(servername string) (tlsConfig *tls.Config, err error) {
	root, err := Asset("assets/tls/trusted.root.crt")
	if err != nil {
		return
	}

	return clientConfigFromBytes(servername, [][]byte{root})
}

func clientConfigFromBytes(servername string, roots [][]byte) (*tls.Config, error) {
	pool := x509.NewCertPool()

	for _, rootCrt := range roots {
		pemBlock, _ := pem.Decode(rootCrt)
		if pemBlock == nil {
			return nil, fmt.Errorf("Bad PEM data")
		}

		certs, err := x509.ParseCertificates(pemBlock.Bytes)
		if err != nil {
			return nil, err
		}

		pool.AddCert(certs[0])
	}

	return &tls.Config{
		RootCAs:    pool,
		ServerName: servername,
	}, nil
}
