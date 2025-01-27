package super

import (
	"testing"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/stretchr/testify/require"
)

func TestResponseToSuper(t *testing.T) {
	t.Run("SingleChain", func(t *testing.T) {
		input := eth.SuperRootResponse{
			Timestamp: 4978924,
			SuperRoot: eth.Bytes32{0x65},
			Chains: []eth.ChainRootInfo{
				{
					ChainID:   eth.ChainID{2987},
					Canonical: eth.Bytes32{0x88},
					Pending:   []byte{1, 2, 3, 4, 5},
				},
			},
		}
		expected := &eth.SuperV1{
			Timestamp: 4978924,
			Chains: []eth.ChainIDAndOutput{
				{ChainID: eth.ChainIDFromUInt64(2987), Output: eth.Bytes32{0x88}},
			},
		}
		actual := responseToSuper(input)
		require.Equal(t, expected, actual)
	})

	t.Run("SortChainsByChainID", func(t *testing.T) {
		input := eth.SuperRootResponse{
			Timestamp: 4978924,
			SuperRoot: eth.Bytes32{0x65},
			Chains: []eth.ChainRootInfo{
				{
					ChainID:   eth.ChainID{2987},
					Canonical: eth.Bytes32{0x88},
					Pending:   []byte{1, 2, 3, 4, 5},
				},
				{
					ChainID:   eth.ChainID{100},
					Canonical: eth.Bytes32{0x10},
					Pending:   []byte{1, 2, 3, 4, 5},
				},
			},
		}
		expected := &eth.SuperV1{
			Timestamp: 4978924,
			Chains: []eth.ChainIDAndOutput{
				{ChainID: eth.ChainIDFromUInt64(100), Output: eth.Bytes32{0x10}},
				{ChainID: eth.ChainIDFromUInt64(2987), Output: eth.Bytes32{0x88}},
			},
		}
		actual := responseToSuper(input)
		require.Equal(t, expected, actual)
	})
}
