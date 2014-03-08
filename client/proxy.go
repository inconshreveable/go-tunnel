package client

import (
	"bufio"
	"code.google.com/p/go.net/proxy"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"github.com/inconshreveable/muxado"
	"net"
	"net/http"
	"net/url"
)

func SOCKS5Dialer(network, proxyAddr, user, password, addr string, tlsConfig *tls.Config) func () (muxado.Session, error) {
    return func() (muxado.Session, error) {
        proxyAuth := &proxy.Auth{User: user, Password: password}

        proxyDialer, err := proxy.SOCKS5(network, proxyAddr, proxyAuth, proxy.Direct)
        if err != nil {
            return nil, err
        }

        conn, err := proxyDialer.Dial("tcp", addr)
        if err != nil {
            return nil, err
        }

        // upgrade to TLS
        conn = tls.Client(conn, tlsConfig)

        return muxado.Client(conn), nil
    }
}

func HTTPDialer(network, proxyUrl, addr string, tlsConfig *tls.Config) func() (muxado.Session, error) {
    return func() (sess muxado.Session, err error) {
        // parse the proxy address
        var parsedUrl *url.URL
        if parsedUrl, err = url.Parse(proxyUrl); err != nil {
            return
        }

        var proxyAuth string
        if parsedUrl.User != nil {
            proxyAuth = "Basic " + base64.StdEncoding.EncodeToString([]byte(parsedUrl.User.String()))
        }

        var dial func(net, addr string) (net.Conn, error)
        switch parsedUrl.Scheme {
        case "http":
            dial = net.Dial
        case "https":
            dial = func(net, addr string) (net.Conn, error) { return tls.Dial(net, addr, new(tls.Config)) }
        default:
            err = fmt.Errorf("Proxy URL scheme must be http or https, got: %s", parsedUrl.Scheme)
            return
        }

        // dial the proxy
        var conn net.Conn
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
        conn = tls.Client(conn, tlsConfig)

        return muxado.Client(conn), nil
    }
}
