package conn

import (
	"crypto/tls"
	log "github.com/inconshreveable/go-tunnel/log"
	util "github.com/inconshreveable/go-tunnel/util"
	"io"
	"net"
	"sync"
)

var (
	logger = log.NewTaggedLogger("go-tunnel/conn")
)

type Conn interface {
	net.Conn
	log.Logger
}

type Logged struct {
	net.Conn
	log.Logger
}

type Listener struct {
	net.Addr
	Conns chan *Logged
}

func wrapConn(conn net.Conn, tags ...string) *Logged {
	switch c := conn.(type) {
	case *Logged:
		return c
	default:
		logged := &Logged{Conn: conn, Logger: log.NewTaggedLogger(util.RandId(4))}
		logged.AddTags(tags...)
		return logged
	}

	return nil
}

func Listen(addr, typ string, tlsCfg *tls.Config) (l *Listener, err error) {
	// listen for incoming connections
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return
	}

	l = &Listener{
		Addr:  listener.Addr(),
		Conns: make(chan *Logged),
	}

	go func() {
		for {
			rawConn, err := listener.Accept()
			if err != nil {
				logger.Error("Failed to accept new TCP connection of type %s: %v", typ, err)
				continue
			}

			c := wrapConn(rawConn, typ)
			if tlsCfg != nil {
				c.Conn = tls.Server(c.Conn, tlsCfg)
			}
			c.Info("New connection from %v", c.RemoteAddr())
			l.Conns <- c
		}
	}()
	return
}

func Wrap(conn net.Conn, tags ...string) *Logged {
	return wrapConn(conn, tags...)
}

func (c *Logged) SetTags(tags ...string) {
	c.Logger = log.NewTaggedLogger(tags...)
}

func (c *Logged) AddTags(tags ...string) {
	c.Logger.AddTags(tags...)
}

func (c *Logged) StartTLS(tlsCfg *tls.Config) {
	c.Conn = tls.Client(c.Conn, tlsCfg)
}

func (c *Logged) Close() (err error) {
	if err := c.Conn.Close(); err == nil {
		c.Debug("Closing")
	}
	return
}

func (c *Logged) Id() string {
	return c.Logger.Name()
}

func Join(c Conn, c2 Conn) (int64, int64) {
	var wait sync.WaitGroup

	pipe := func(to Conn, from Conn, bytesCopied *int64) {
		defer to.Close()
		defer from.Close()
		defer wait.Done()

		var err error
		*bytesCopied, err = io.Copy(to, from)
		if err != nil {
			from.Warn("Copied %d bytes to %s before failing with error %v", *bytesCopied, to.Name(), err)
		} else {
			from.Debug("Copied %d bytes to %s", *bytesCopied, to.Name())
		}
	}

	wait.Add(2)
	var fromBytes, toBytes int64
	go pipe(c, c2, &fromBytes)
	go pipe(c2, c, &toBytes)
	c.Info("Joined with connection %s", c2.Name())
	wait.Wait()
	return fromBytes, toBytes
}
