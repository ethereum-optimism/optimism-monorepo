package types

import (
	"context"
	"crypto/ecdsa"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

type Address = common.Address

type ChainID = *big.Int

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

type Key = *ecdsa.PrivateKey
