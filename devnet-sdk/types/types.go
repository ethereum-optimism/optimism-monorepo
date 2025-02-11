package types

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

type Address = common.Address

type ChainID *big.Int

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
