# go-tunnel

Listen for connections made to a remote machine.

Your typical net.Listen() call lets you listen for new connections on a local port. go-tunnel
provides an abstraction that lets your application listen for new connections on a port on a remote
machine. It also has special support for HTTP listening which allows a client to ask to listen for
connections just to a specific HTTP hostname (or on a random one).

## The demo

We're going to serve a simple HTTP service on a public URL by just adding a single library call.

Here's a simple Go HTTP server that listens on port 9090 of your local machine.

	package main

	import (
		"net/http"
		"io"
		"fmt"
	)

	func main() {
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			fmt.Printf("Serving hello world page!\n")
			io.WriteString(w, "Hello world!\n")
		})

		err = http.ServeAndListen(":9090")
		if err != nil {
			panic(err)
		}
	}

Here's the same simple Go HTTP server that listens on a randomly-assigned public hostname
assigned by the free service I provide on airlock.io:

	package main

	import (
		"net/http"
		"io"
		"fmt"
		"github.com/inconshreveable/go-tunnel"
	)

	func main() {
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			fmt.Printf("Serving hello world page!\n")
			io.WriteString(w, "Hello world!\n")
		})

		tun, err := tunnel.ListenHTTP()
		if err != nil {
			panic(err)
		}

		fmt.Printf("Serving at: %v\n", tun.Addr())

		err = http.Serve(tun, nil)
		if err != nil {
			panic(err)
		}
	}

Instead of calling http.ListenAndServe, you call http.Serve and pass in the tunnel listener
you create by a call to tunnel.ListenHTTP()

## The Tunnel object

The coolest part of the library is that calls to Session.Listen(), Session.Listen\*(), as well as the top
level calls ListenHTTP() and ListenTCP() return a Tunnel object.

Tunnel objects *implement net.Listener*. This means you can pass them into http.Serve like in the example above. You
can accept new connections from them. If you print the Tunnel.Addr(), you'll get the public address you can
connect to the tunnel with.

The connections returned by the Tunnel.Accept() method even behave like they were accepted on the remote machine.
If you print .RemoteAddr() on a connection accepted from a tunnel, you will get back the address of *the remote address of
the public connection*.

.Close() a tunnel to return the port or virtual hostname to the server so it may be allocated to another client,
just like you would with a TCP net.Listener.

## Tunneling over a custom server

If you want to connect to a custom server instead of using the free public server, you'll need to first
create a session to a different tunneling service.

	sess, err := tunnel.Dial("example.com:1234")
	if err != nil {
		panic(err)
	}

	// now bind for remote connections
	tun, err := sess.ListenTCP(tcpOptions)

## Customizing listen calls

The Listen calls on a Session are a little more flexible than the top-level calls. They let you listen with a set of 
options that can customize the tunnel you bind.

### Binding a specific port

For TCP tunnels, you may request the server bind a specific port for you instead of a random one:

	import (
		"github.com/inconshreveable/go-tunnel"
		"github.com/inconshreveable/go-tunnel/proto"
	)

	func main() {
		sess, err := tunnel.Dial("example.com:1234")
		if err != nil {
			panic(err)
		}

		// now bind for remote connections
		tun, err := sess.ListenTCP(&proto.TCPOptions{RemotePort: 12345})
	}

### Binding with a specific subdomain and authentication
	import (
		"github.com/inconshreveable/go-tunnel"
		"github.com/inconshreveable/go-tunnel/proto"
	)

	func main() {
		sess, err := tunnel.Dial("example.com:1234")
		if err != nil {
			panic(err)
		}

		// now bind for remote connections
		tun, err := sess.ListenHTTP(&proto.HTTPPOptions{Auth: "user:secret", Subdomain: "example"})
	}

## Custom tunnel servers with the server library

The go-tunnel library also has code that lets you create custom tunneling servers. Typical server
setup involves creating a set of binders, and then running the server. This setup would set up a
tunnel server that listenson port 12345 for new TLS-encrypted tunnel sessions and allows binding
http tunnels only:

	import (
		"github.com/inconshreveable/go-tunnel/server"
		"github.com/inconshreveable/go-tunnel/server/binder"
	)

	func main() {
		binders := make(map[string] binders.Binder)

		if binders["http"], err = binder.NewHTTPBinder(":80", "example.com", 10 * time.Second); err != nil {
			panic(err)
		}

		server, err := server.ServeTLS("tcp", ":12345", exampleDotComTlsConfig, binders)
		if err != nil {
			panic(err)
		}

		server.Run()
	}


You can inject custom behavior into the tunneling server by creating a custom set of *server.SessionHooks* and
*server.TunnelHooks* and setting those properties on your Server object.

## API Documentation

API documentation is available on godoc:
[https://godoc.org/github.com/inconshreveable/go-tunnel](https://godoc.org/github.com/inconshreveable/go-tunnel)
