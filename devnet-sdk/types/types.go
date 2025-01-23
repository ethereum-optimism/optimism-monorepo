package types

import (
	"context"
	"fmt"
	"log/slog"
)

type Address string

type Balance uint64

// LogValue implements slog.LogValuer to format Balance in the most readable unit
func (b Balance) LogValue() slog.Value {
	val := float64(b)

	// 1 ETH = 1e18 Wei
	if val >= 0.001e18 {
		return slog.StringValue(fmt.Sprintf("%.3g ETH", val/1e18))
	}

	// 1 Gwei = 1e9 Wei
	if val >= 0.001e9 {
		return slog.StringValue(fmt.Sprintf("%.3g Gwei", val/1e9))
	}

	// Wei
	return slog.StringValue(fmt.Sprintf("%d Wei", uint64(val)))
}

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
}

type Key = string
