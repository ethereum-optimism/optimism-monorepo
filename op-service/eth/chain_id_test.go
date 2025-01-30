package eth

import (
	"math"
	"testing"

	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func TestChainID_String(t *testing.T) {
	tests := []struct {
		input    ChainID
		expected string
	}{
		{ChainIDFromUInt64(0), "0"},
		{ChainIDFromUInt64(1), "1"},
		{ChainIDFromUInt64(871975192374), "871975192374"},
		{ChainIDFromUInt64(math.MaxInt64), "9223372036854775807"},
		{ChainID(*uint256.NewInt(math.MaxUint64)), "18446744073709551615"},
		{ChainID(*uint256.MustFromDecimal("1844674407370955161618446744073709551616")), "1844674407370955161618446744073709551616"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.expected, func(t *testing.T) {
			t.Run("String", func(t *testing.T) {
				require.Equal(t, test.expected, test.input.String())
			})
			t.Run("MarshalText", func(t *testing.T) {
				data, err := test.input.MarshalText()
				require.NoError(t, err)
				require.Equal(t, test.expected, string(data))
			})
			t.Run("UnmarshalText", func(t *testing.T) {
				var id ChainID
				require.NoError(t, id.UnmarshalText([]byte(test.expected)))
				require.Equal(t, test.input, id)
			})
		})
	}
}
