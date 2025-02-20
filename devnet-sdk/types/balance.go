package types

import (
	"fmt"
	"log/slog"
	"math/big"
)

type Balance struct {
	*big.Int
}

// NewBalance creates a new Balance from a big.Int
func NewBalance(i *big.Int) Balance {
	return Balance{Int: new(big.Int).Set(i)}
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

// GreaterThan returns true if this balance is greater than other
func (b Balance) GreaterThan(other Balance) bool {
	if b.Int == nil {
		return false
	}
	if other.Int == nil {
		return true
	}
	return b.Int.Cmp(other.Int) > 0
}

// LessThan returns true if this balance is less than other
func (b Balance) LessThan(other Balance) bool {
	if b.Int == nil {
		return other.Int != nil
	}
	if other.Int == nil {
		return false
	}
	return b.Int.Cmp(other.Int) < 0
}

// Equal returns true if this balance equals other
func (b Balance) Equal(other Balance) bool {
	if b.Int == nil {
		return other.Int == nil
	}
	if other.Int == nil {
		return false
	}
	return b.Int.Cmp(other.Int) == 0
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
		str := eth.Text('f', 0)
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
