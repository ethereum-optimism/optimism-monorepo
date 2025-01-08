package testutils

import (
	"context"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/ethereum-optimism/optimism/op-service/client"
)

// RPCErrFaker implements an RPC by wrapping one, but returns an error when prepared with one, to test RPC error handling.
type RPCErrFaker struct {
	// RPC to call when no ErrFn is set, or the ErrFn does not return an error
	RPC client.RPC
	// ErrFn returns an error when the RPC needs to return error upon a call, batch call or subscription (nil input).
	// The RPC operates without fake errors if the ErrFn is nil, or returns nil.
	ErrFn func(call []rpc.BatchElem) error
}

func (r RPCErrFaker) Close() {
	r.RPC.Close()
}

func (r RPCErrFaker) CallContext(ctx context.Context, result any, method string, args ...any) error {
	if r.ErrFn != nil {
		if err := r.ErrFn([]rpc.BatchElem{{
			Method: method,
			Args:   args,
			Result: result,
		}}); err != nil {
			return err
		}
	}
	return r.RPC.CallContext(ctx, result, method, args...)
}

func (r RPCErrFaker) BatchCallContext(ctx context.Context, b []rpc.BatchElem) error {
	if r.ErrFn != nil {
		if err := r.ErrFn(b); err != nil {
			return err
		}
	}
	return r.RPC.BatchCallContext(ctx, b)
}

func (r RPCErrFaker) Subscribe(ctx context.Context, namespace string, channel any, args ...any) (ethereum.Subscription, error) {
	if r.ErrFn != nil {
		if err := r.ErrFn(nil); err != nil {
			return nil, err
		}
	}
	return r.RPC.Subscribe(ctx, namespace, channel, args...)
}

var _ client.RPC = (*RPCErrFaker)(nil)
