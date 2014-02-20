package proto

import (
	"encoding/json"
	"reflect"
)

var TypeMap map[string]reflect.Type

func init() {
	TypeMap = make(map[string]reflect.Type)

	t := func(obj interface{}) reflect.Type { return reflect.TypeOf(obj).Elem() }
	TypeMap["Auth"] = t((*Auth)(nil))
	TypeMap["AuthResp"] = t((*AuthResp)(nil))
	TypeMap["Bind"] = t((*Bind)(nil))
	TypeMap["BindResp"] = t((*BindResp)(nil))
	TypeMap["StartProxy"] = t((*StartProxy)(nil))
}

type Message interface{}

type Envelope struct {
	Type    string
	Payload json.RawMessage
}

// When a client opens a new control channel to the server
// it must start by sending an Auth message.
type Auth struct {
	Version  []string    // protocol versions supported, ordered by preference
	ClientId string      // empty for new sessions
	Extra    interface{} // clients may add whatever data the like to auth messages
}

// A server responds to an Auth message with an
// AuthResp message over the control channel.
//
// If Error is not the empty string
// the server has indicated it will not accept
// the new session and will close the connection.
//
// The server response includes a unique ClientId
// that is used to associate and authenticate future
// proxy connections via the same field in RegProxy messages.
type AuthResp struct {
	Version  string // protocol version chosen
	ClientId string
	Error    string
	Extra    interface{}
}

// A client sends this message to the server over a new stream
// to request the server bind a remote port/hostname on the client's behalf.
type Bind struct {
	Protocol string      // the protocol to bind
	Options  interface{} // options for the bind - protocol dependent
	Extra    interface{} // anything extra the application wants to send
}

type HTTPOptions struct {
	Hostname  string
	Subdomain string
	Auth      string
}

type TCPOptions struct {
	RemotePort uint16
}

type TLSOptions struct {
	Hostname  string
	Subdomain string
}

// The server responds with a BindResp message to notify the client
// of the success or failure of a bind.
type BindResp struct {
	Url      string
	Protocol string
	Error    string
	Extra    interface{}
}

// A client sends this message to the server over a new stream
// to request the server unbind a previously bound tunnel
type Unbind struct {
	Url   string
	Extra interface{}
}

// The server response with an UnbindResp message to notify the client
// of the success or failure of the unbind.
type UnbindResp struct {
	Error string
	Extra interface{}
}

// This message is sent first over a new stream from the server to the client to
// provide it with metadata about the connection it will tunnel over the stream.
type StartProxy struct {
	Url        string // URL of the tunnel this connection connection is being proxied for
	ClientAddr string // Network address of the client initiating the connection to the tunnel
}
