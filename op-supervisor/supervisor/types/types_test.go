package types

import (
	"encoding/json"
	"math"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
)

func FuzzRoundtripIdentifierJSONMarshal(f *testing.F) {
	f.Fuzz(func(t *testing.T, origin []byte, blockNumber uint64, logIndex uint32, timestamp uint64, chainID []byte) {
		if len(chainID) > 32 {
			chainID = chainID[:32]
		}

		id := Identifier{
			Origin:      common.BytesToAddress(origin),
			BlockNumber: blockNumber,
			LogIndex:    logIndex,
			Timestamp:   timestamp,
			ChainID:     ChainIDFromBig(new(big.Int).SetBytes(chainID)),
		}

		raw, err := json.Marshal(&id)
		require.NoError(t, err)

		var dec Identifier
		require.NoError(t, json.Unmarshal(raw, &dec))

		require.Equal(t, id.Origin, dec.Origin)
		require.Equal(t, id.BlockNumber, dec.BlockNumber)
		require.Equal(t, id.LogIndex, dec.LogIndex)
		require.Equal(t, id.Timestamp, dec.Timestamp)
		require.Equal(t, id.ChainID, dec.ChainID)
	})
}

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
