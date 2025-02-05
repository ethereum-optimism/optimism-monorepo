package types

import (
	"math/big"
	"testing"
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

func TestBalance_Comparisons(t *testing.T) {
	tests := []struct {
		a, b       int64
		gt, lt, eq bool
	}{
		{100, 200, false, true, false},
		{200, 100, true, false, false},
		{100, 100, false, false, true},
		{0, 100, false, true, false},
	}

	for _, tt := range tests {
		a := NewBalance(big.NewInt(tt.a))
		b := NewBalance(big.NewInt(tt.b))

		if got := a.GreaterThan(b); got != tt.gt {
			t.Errorf("GreaterThan(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.gt)
		}

		if got := a.LessThan(b); got != tt.lt {
			t.Errorf("LessThan(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.lt)
		}

		if got := a.Equal(b); got != tt.eq {
			t.Errorf("Equal(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.eq)
		}
	}
}

func TestBalance_LogValue(t *testing.T) {
	tests := []struct {
		wei  string // Using string to handle large numbers
		want string
	}{
		{"2000000000000000000", "2 ETH"},   // 2 ETH
		{"1000000000", "1 Gwei"},           // 1 Gwei
		{"100", "100 Wei"},                 // 100 Wei
		{"1500000000000000000", "1.5 ETH"}, // 1.5 ETH
		{"0", "0 Wei"},                     // 0
	}

	for _, tt := range tests {
		i := new(big.Int)
		i.SetString(tt.wei, 10)
		b := NewBalance(i)
		got := b.LogValue().String()
		if got != tt.want {
			t.Errorf("LogValue() for %v Wei = %v, want %v", tt.wei, got, tt.want)
		}
	}

	// Test nil case
	var nilBalance Balance
	got := nilBalance.LogValue().String()
	if got != "0 ETH" {
		t.Errorf("LogValue() for nil balance = %v, want '0 ETH'", got)
	}
}
