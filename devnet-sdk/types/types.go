package types

import (
	"context"
)

type Address string

type ChainID uint64

type ReadInvocation[T any] interface {
	Call(ctx context.Context) (T, error)
}

type WriteInvocation[T any] interface {
	ReadInvocation[T]
	Send(ctx context.Context) InvocationResult
}

type InvocationResult interface {
	Error() error
	Wait() error
}

type Wallet interface {
	PrivateKey() Key
	Address() Address
	SendETH(to Address, amount Balance) WriteInvocation[any]
	Balance() Balance
}

type Key = string
