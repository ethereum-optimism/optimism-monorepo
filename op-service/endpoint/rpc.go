package endpoint

import (
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/rpc"
)

type MustRPC struct {
	Value RPC
}

func (u *MustRPC) UnmarshalText(data []byte) error {
	if u == nil {
		return fmt.Errorf("cannot unmarshal %q into nil MustRPC", string(data))
	}
	v := URL(data)
	if v == "" {
		return errors.New("empty RPC URL")
	}
	u.Value = v
	return nil
}

func (u MustRPC) MarshalText() ([]byte, error) {
	if u.Value == nil {
		return nil, errors.New("missing RPC")
	}
	out := u.Value.RPC()
	if out == "" {
		return nil, errors.New("missing RPC")
	}
	return []byte(out), nil
}

type OptionalRPC struct {
	Value RPC
}

func (u *OptionalRPC) UnmarshalText(data []byte) error {
	if u == nil {
		return fmt.Errorf("cannot unmarshal %q into nil OptionalRPC", string(data))
	}
	u.Value = URL(data)
	return nil
}

func (u OptionalRPC) MarshalText() ([]byte, error) {
	if u.Value == nil {
		return []byte{}, nil
	}
	out := u.Value.RPC()
	return []byte(out), nil
}

// RPC is an interface for an endpoint to provide flexibility.
// By default the RPC just returns an RPC endpoint string.
// But the RPC can implement one or more extension interfaces,
// to provide alternative ways of establishing a connection,
// or even a fully initialized client binding.
type RPC interface {
	RPC() string
}

// URL is a generic RPC endpoint URL
type URL string

var _ RPC = URL("")

func (u URL) RPC() string {
	return string(u)
}

// WsRPC is an RPC extension interface,
// to explicitly provide the Websocket RPC option.
type WsRPC interface {
	RPC
	WsRPC() string
}

// HttpRPC is an RPC extension interface,
// to explicitly provide the HTTP RPC option.
type HttpRPC interface {
	RPC
	HttpRPC() string
}

// ClientRPC is an RPC extension interface,
// providing the option to attach in-process to a client,
// rather than dialing an endpoint.
type ClientRPC interface {
	RPC
	ClientRPC() *rpc.Client
}

// HttpURL is an HTTP endpoint URL
type HttpURL string

func (url HttpURL) RPC() string {
	return string(url)
}

func (url HttpURL) HttpRPC() string {
	return string(url)
}

// WsURL is a websocket endpoint URL
type WsURL string

func (url WsURL) RPC() string {
	return string(url)
}

func (url WsURL) WsRPC() string {
	return string(url)
}

// WsOrHttpRPC provides optionality between
// a websocket RPC endpoint and a HTTP RPC endpoint.
// The default is the websocket endpoint.
type WsOrHttpRPC struct {
	WsURL   string
	HttpURL string
}

func (r WsOrHttpRPC) RPC() string {
	return r.WsURL
}

func (r WsOrHttpRPC) WsRPC() string {
	return r.WsURL
}

func (r WsOrHttpRPC) HttpRPC() string {
	return r.HttpURL
}

// ServerRPC is a very flexible RPC: it can attach in-process to a server,
// or select one of the fallback RPC methods.
type ServerRPC struct {
	Fallback WsOrHttpRPC
	Server   *rpc.Server
}

func (e *ServerRPC) RPC() string {
	return e.Fallback.RPC()
}

func (e *ServerRPC) WsRPC() string {
	return e.Fallback.WsRPC()
}

func (e *ServerRPC) HttpRPC() string {
	return e.Fallback.HttpRPC()
}

func (e *ServerRPC) ClientRPC() *rpc.Client {
	return rpc.DialInProc(e.Server)
}

type Dialer func(v string) *rpc.Client

type RPCPreference int

const (
	PreferAnyRPC RPCPreference = iota
	PreferHttpRPC
	PreferWSRPC
)

// DialRPC navigates the RPC interface,
// to find the optimal version of the PRC to dial or attach to.
func DialRPC(preference RPCPreference, rpc RPC, dialer Dialer) *rpc.Client {
	if v, ok := rpc.(HttpRPC); preference == PreferHttpRPC && ok {
		return dialer(v.HttpRPC())
	}
	if v, ok := rpc.(WsRPC); preference == PreferWSRPC && ok {
		return dialer(v.WsRPC())
	}
	if v, ok := rpc.(ClientRPC); ok {
		return v.ClientRPC()
	}
	return dialer(rpc.RPC())
}

// SelectRPC selects an endpoint URL, based on preference.
// For more optimal dialing, use DialRPC.
func SelectRPC(preference RPCPreference, rpc RPC) string {
	if v, ok := rpc.(HttpRPC); preference == PreferHttpRPC && ok {
		return v.HttpRPC()
	}
	if v, ok := rpc.(WsRPC); preference == PreferWSRPC && ok {
		return v.WsRPC()
	}
	return rpc.RPC()
}
