package conn

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"net"
	"bufio"
	"encoding/base64"
	"code.google.com/p/go.net/proxy"
)


type Dialer func(network, addr string) (*Logged, error)

func TCPDialer(tags... string) Dialer {
	return func(network, addr string) (conn *Logged, err error) {
		var rawConn net.Conn
		if rawConn, err = net.Dial(network, addr); err != nil {
			return
		}

		conn = wrapConn(rawConn, tags...)
		conn.Debug("New connection to: %v", rawConn.RemoteAddr())
		return
	}
}

func TLSDialer(tlsConfig *tls.Config, tags... string) Dialer {
	dialer := TCPDialer(tags...)
	return func(network, addr string) (*Logged, error) {
		conn, err := dialer(network, addr)
		if err != nil {
			return nil, err
		}

		conn.StartTLS(tlsConfig)
		return conn, nil
	}
}

func HttpTLSDialer(proxyUrl string, tlsCfg *tls.Config, tags... string) Dialer {
	return func(network, addr string) (conn *Logged, err error) {
		// parse the proxy address
		var parsedUrl *url.URL
		if parsedUrl, err = url.Parse(proxyUrl); err != nil {
			return
		}

		var proxyAuth string
		if parsedUrl.User != nil {
			proxyAuth = "Basic " + base64.StdEncoding.EncodeToString([]byte(parsedUrl.User.String()))
		}

		var dial Dialer
		switch parsedUrl.Scheme {
		case "http":
			dial = TCPDialer(tags...)
		case "https":
			// use host TLS configuration for resolving the proxy
			dial = TLSDialer(new(tls.Config), tags...)
		default:
			err = fmt.Errorf("Proxy URL scheme must be http or https, got: %s", parsedUrl.Scheme)
			return
		}

		// dial the proxy
		if conn, err = dial(network, parsedUrl.Host); err != nil {
			return
		}

		// send an HTTP proxy CONNECT message
		req, err := http.NewRequest("CONNECT", "https://"+addr, nil)
		if err != nil {
			return
		}

		if proxyAuth != "" {
			req.Header.Set("Proxy-Authorization", proxyAuth)
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; ngrok)")
		req.Write(conn)

		// read the proxy's response
		resp, err := http.ReadResponse(bufio.NewReader(conn), req)
		if err != nil {
			return
		}
		resp.Body.Close()

		if resp.StatusCode != 200 {
			err = fmt.Errorf("Non-200 response from proxy server: %s", resp.Status)
			return
		}

		// upgrade to TLS
		conn.StartTLS(tlsCfg)
		return
	}
}

func SOCKS5TLSDialer(proxyNetwork, proxyAddr string, tlsConfig *tls.Config, tags... string) Dialer {
	return func(network, addr string) (conn *Logged, err error) {
		d, err := proxy.SOCKS5(proxyNetwork, proxyAddr, nil, proxy.Direct)
		if err != nil {
			return
		}

		raw, err := d.Dial(network, addr)
		if err != nil {
			return
		}
		conn = wrapConn(raw, tags...)
		conn.StartTLS(tlsConfig)
		return
	}
}

