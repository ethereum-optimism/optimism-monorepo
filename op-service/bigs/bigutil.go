package bigs

import "math/big"

func Equal(a *big.Int, b *big.Int) bool {
	return a.Cmp(b) == 0
}

func IsZero(val *big.Int) bool {
	return val.Cmp(big.NewInt(0)) == 0
}

func IsPositive(val *big.Int) bool {
	return val.Cmp(big.NewInt(0)) > 0
}

func IsNegative(val *big.Int) bool {
	return val.Cmp(big.NewInt(0)) < 0
}
