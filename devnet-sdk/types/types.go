package types

import (
	"context"
	"fmt"
	"log/slog"
	"math/big"
)

type Address string

type Balance struct {
	*big.Int
}

// NewBalance creates a new Balance from a big.Int
func NewBalance(i *big.Int) Balance {
	return Balance{Int: new(big.Int).Set(i)}
}

// NewBalanceFromFloat creates a Balance from a float64 value in ETH
func NewBalanceFromFloat(eth float64) Balance {
	// Convert ETH to Wei (1 ETH = 1e18 Wei)
	wei := new(big.Float).Mul(new(big.Float).SetFloat64(eth), new(big.Float).SetFloat64(1e18))

	// Convert to big.Int (truncating any fractional part)
	result := new(big.Int)
	wei.Int(result)
	return Balance{Int: result}
}

// Add returns a new Balance with other added to it
func (b Balance) Add(other Balance) Balance {
	return Balance{Int: new(big.Int).Add(b.Int, other.Int)}
}

// Sub returns a new Balance with other subtracted from it
func (b Balance) Sub(other Balance) Balance {
	return Balance{Int: new(big.Int).Sub(b.Int, other.Int)}
}

// Mul returns a new Balance multiplied by a float64
func (b Balance) Mul(f float64) Balance {
	floatResult := new(big.Float).Mul(new(big.Float).SetInt(b.Int), new(big.Float).SetFloat64(f))
	result := new(big.Int)
	floatResult.Int(result)
	return Balance{Int: result}
}

// LogValue implements slog.LogValuer to format Balance in the most readable unit
func (b Balance) LogValue() slog.Value {
	if b.Int == nil {
		return slog.StringValue("0 ETH")
	}

	val := new(big.Float).SetInt(b.Int)
	eth := new(big.Float).Quo(val, new(big.Float).SetInt64(1e18))

	// 1 ETH = 1e18 Wei
	if eth.Cmp(new(big.Float).SetFloat64(0.001)) >= 0 {
		str := eth.Text('g', 3)
		return slog.StringValue(fmt.Sprintf("%s ETH", str))
	}

	// 1 Gwei = 1e9 Wei
	gwei := new(big.Float).Quo(val, new(big.Float).SetInt64(1e9))
	if gwei.Cmp(new(big.Float).SetFloat64(0.001)) >= 0 {
		str := gwei.Text('g', 3)
		return slog.StringValue(fmt.Sprintf("%s Gwei", str))
	}

	// Wei
	return slog.StringValue(fmt.Sprintf("%s Wei", b.Text(10)))
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
