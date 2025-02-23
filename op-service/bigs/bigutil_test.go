package bigs

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEqual(t *testing.T) {
	require.True(t, Equal(big.NewInt(0), big.NewInt(0)))
	require.True(t, Equal(big.NewInt(1), big.NewInt(1)))
	require.True(t, Equal(big.NewInt(1900), big.NewInt(1900)))
	require.True(t, Equal(big.NewInt(-1), big.NewInt(-1)))
	require.True(t, Equal(big.NewInt(-1900), big.NewInt(-1900)))

	require.False(t, Equal(big.NewInt(0), big.NewInt(1)))
	require.False(t, Equal(big.NewInt(1), big.NewInt(0)))
	require.False(t, Equal(big.NewInt(1), big.NewInt(2)))
	require.False(t, Equal(big.NewInt(-1900), big.NewInt(1900)))
}

func TestIsZero(t *testing.T) {
	require.True(t, IsZero(big.NewInt(0)))
	require.False(t, IsZero(big.NewInt(1)))
	require.False(t, IsZero(big.NewInt(-1)))
}

func TestIsPositive(t *testing.T) {
	require.True(t, IsPositive(big.NewInt(1)))
	require.True(t, IsPositive(big.NewInt(2)))

	require.False(t, IsPositive(big.NewInt(0)))
	require.False(t, IsPositive(big.NewInt(-1)))
}

func TestIsNegative(t *testing.T) {
	require.True(t, IsNegative(big.NewInt(-1)))
	require.True(t, IsNegative(big.NewInt(-2)))

	require.False(t, IsNegative(big.NewInt(0)))
	require.False(t, IsNegative(big.NewInt(1)))
}
