package types

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewBalance(t *testing.T) {
	i := big.NewInt(100)
	b := NewBalance(i)
	if b.Int.Cmp(i) != 0 {
		t.Errorf("NewBalance failed, got %v, want %v", b.Int, i)
	}

	// Verify that modifying the input doesn't affect the Balance
	i.SetInt64(200)
	if b.Int.Cmp(big.NewInt(100)) != 0 {
		t.Error("NewBalance did not create a copy of the input")
	}
}

func TestBalance_Add(t *testing.T) {
	tests := []struct {
		a, b, want int64
	}{
		{100, 200, 300},
		{0, 100, 100},
		{-100, 100, 0},
		{1000000, 2000000, 3000000},
	}

	for _, tt := range tests {
		a := NewBalance(big.NewInt(tt.a))
		b := NewBalance(big.NewInt(tt.b))
		got := a.Add(b)
		want := NewBalance(big.NewInt(tt.want))
		if !got.Equal(want) {
			t.Errorf("Add(%v, %v) = %v, want %v", tt.a, tt.b, got, want)
		}
		// Verify original balances weren't modified
		if !a.Equal(NewBalance(big.NewInt(tt.a))) {
			t.Error("Add modified original balance")
		}
	}
}

func TestBalance_Sub(t *testing.T) {
	tests := []struct {
		a, b, want int64
	}{
		{300, 200, 100},
		{100, 100, 0},
		{0, 100, -100},
		{3000000, 2000000, 1000000},
	}

	for _, tt := range tests {
		a := NewBalance(big.NewInt(tt.a))
		b := NewBalance(big.NewInt(tt.b))
		got := a.Sub(b)
		want := NewBalance(big.NewInt(tt.want))
		if !got.Equal(want) {
			t.Errorf("Sub(%v, %v) = %v, want %v", tt.a, tt.b, got, want)
		}
	}
}

func TestBalance_Mul(t *testing.T) {
	tests := []struct {
		a    int64
		mul  float64
		want int64
	}{
		{100, 2.0, 200},
		{100, 0.5, 50},
		{100, 0.0, 0},
		{1000, 1.5, 1500},
	}

	for _, tt := range tests {
		a := NewBalance(big.NewInt(tt.a))
		got := a.Mul(tt.mul)
		want := NewBalance(big.NewInt(tt.want))
		if !got.Equal(want) {
			t.Errorf("Mul(%v, %v) = %v, want %v", tt.a, tt.mul, got, want)
		}
	}
}

func TestBalanceComparisons(t *testing.T) {
	tests := []struct {
		name     string
		balance1 Balance
		balance2 Balance
		greater  bool
		less     bool
		equal    bool
	}{
		{
			name:     "both nil",
			balance1: Balance{},
			balance2: Balance{},
			greater:  false,
			less:     false,
			equal:    true,
		},
		{
			name:     "first nil",
			balance1: Balance{},
			balance2: NewBalance(big.NewInt(100)),
			greater:  false,
			less:     true,
			equal:    false,
		},
		{
			name:     "second nil",
			balance1: NewBalance(big.NewInt(100)),
			balance2: Balance{},
			greater:  true,
			less:     false,
			equal:    false,
		},
		{
			name:     "first greater",
			balance1: NewBalance(big.NewInt(200)),
			balance2: NewBalance(big.NewInt(100)),
			greater:  true,
			less:     false,
			equal:    false,
		},
		{
			name:     "second greater",
			balance1: NewBalance(big.NewInt(100)),
			balance2: NewBalance(big.NewInt(200)),
			greater:  false,
			less:     true,
			equal:    false,
		},
		{
			name:     "equal values",
			balance1: NewBalance(big.NewInt(100)),
			balance2: NewBalance(big.NewInt(100)),
			greater:  false,
			less:     false,
			equal:    true,
		},
		{
			name:     "zero values",
			balance1: NewBalance(new(big.Int)),
			balance2: NewBalance(new(big.Int)),
			greater:  false,
			less:     false,
			equal:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.greater, tt.balance1.GreaterThan(tt.balance2), "GreaterThan check failed")
			assert.Equal(t, tt.less, tt.balance1.LessThan(tt.balance2), "LessThan check failed")
			assert.Equal(t, tt.equal, tt.balance1.Equal(tt.balance2), "Equal check failed")
		})
	}
}

func TestBalanceArithmetic(t *testing.T) {
	tests := []struct {
		name     string
		balance1 Balance
		balance2 Balance
		add      *big.Int
		sub      *big.Int
		mul      float64
		mulRes   *big.Int
	}{
		{
			name:     "basic arithmetic",
			balance1: NewBalance(big.NewInt(100)),
			balance2: NewBalance(big.NewInt(50)),
			add:      big.NewInt(150),
			sub:      big.NewInt(50),
			mul:      2.5,
			mulRes:   big.NewInt(250),
		},
		{
			name:     "zero values",
			balance1: NewBalance(new(big.Int)),
			balance2: NewBalance(new(big.Int)),
			add:      new(big.Int),
			sub:      new(big.Int),
			mul:      1.0,
			mulRes:   new(big.Int),
		},
		{
			name:     "large numbers",
			balance1: NewBalance(new(big.Int).Mul(big.NewInt(1e18), big.NewInt(100))), // 100 ETH
			balance2: NewBalance(new(big.Int).Mul(big.NewInt(1e18), big.NewInt(50))),  // 50 ETH
			add:      new(big.Int).Mul(big.NewInt(1e18), big.NewInt(150)),             // 150 ETH
			sub:      new(big.Int).Mul(big.NewInt(1e18), big.NewInt(50)),              // 50 ETH
			mul:      0.5,
			mulRes:   new(big.Int).Mul(big.NewInt(1e18), big.NewInt(50)), // 50 ETH
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test Add
			sum := tt.balance1.Add(tt.balance2)
			assert.Equal(t, 0, sum.Int.Cmp(tt.add), "Add result mismatch")

			// Test Sub
			diff := tt.balance1.Sub(tt.balance2)
			assert.Equal(t, 0, diff.Int.Cmp(tt.sub), "Sub result mismatch")

			// Test Mul
			product := tt.balance1.Mul(tt.mul)
			assert.Equal(t, 0, product.Int.Cmp(tt.mulRes), "Mul result mismatch")
		})
	}
}

func TestBalanceLogValue(t *testing.T) {
	tests := []struct {
		name     string
		balance  Balance
		expected string
	}{
		{
			name:     "nil balance",
			balance:  Balance{},
			expected: "0 ETH",
		},
		{
			name:     "zero balance",
			balance:  NewBalance(new(big.Int)),
			expected: "0 Wei",
		},
		{
			name:     "small wei amount",
			balance:  NewBalance(big.NewInt(100)),
			expected: "100 Wei",
		},
		{
			name:     "gwei amount",
			balance:  NewBalance(new(big.Int).Mul(big.NewInt(1), big.NewInt(1e9))),
			expected: "1 Gwei",
		},
		{
			name:     "eth amount",
			balance:  NewBalance(new(big.Int).Mul(big.NewInt(1), big.NewInt(1e18))),
			expected: "1 ETH",
		},
		{
			name:     "large eth amount",
			balance:  NewBalance(new(big.Int).Mul(big.NewInt(1000), big.NewInt(1e18))),
			expected: "1000 ETH",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logValue := tt.balance.LogValue()
			assert.Equal(t, tt.expected, logValue.String())
		})
	}
}
