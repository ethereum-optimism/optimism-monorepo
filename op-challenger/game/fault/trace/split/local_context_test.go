package split

import (
	"math"
	"math/big"
	"testing"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

func TestCreateLocalContext(t *testing.T) {
	tests := []struct {
		name         string
		preValue     common.Hash
		prePosition  types.Position
		postValue    common.Hash
		postPosition types.Position
		expected     []byte
	}{
		{
			name:         "PreAndPost",
			preValue:     common.HexToHash("abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"),
			prePosition:  types.NewPositionFromGIndex(big.NewInt(2)),
			postValue:    common.HexToHash("cc00000000000000000000000000000000000000000000000000000000000000"),
			postPosition: types.NewPositionFromGIndex(big.NewInt(3)),
			expected:     common.FromHex("abcdef0123456789abcdef0123456789abcdef0123456789abcdef01234567890000000000000000000000000000000000000000000000000000000000000002cc000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000003"),
		},
		{
			name:         "LargePositions",
			preValue:     common.HexToHash("abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"),
			prePosition:  types.NewPositionFromGIndex(new(big.Int).SetBytes(common.FromHex("cbcdef0123456789abcdef0123456789abcdef0123456789abcdef012345678c"))),
			postValue:    common.HexToHash("dd00000000000000000000000000000000000000000000000000000000000000"),
			postPosition: types.NewPositionFromGIndex(new(big.Int).SetUint64(math.MaxUint64)),
			expected:     common.FromHex("abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789cbcdef0123456789abcdef0123456789abcdef0123456789abcdef012345678cdd00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000ffffffffffffffff"),
		},
		{
			name:         "AbsolutePreState",
			preValue:     common.Hash{},
			prePosition:  types.Position{},
			postValue:    common.HexToHash("cc00000000000000000000000000000000000000000000000000000000000000"),
			postPosition: types.NewPositionFromGIndex(big.NewInt(3)),
			expected:     common.FromHex("cc000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000003"),
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			pre := types.Claim{
				ClaimData: types.ClaimData{
					Value:    test.preValue,
					Position: test.prePosition,
				},
			}
			post := types.Claim{
				ClaimData: types.ClaimData{
					Value:    test.postValue,
					Position: test.postPosition,
				},
			}
			actualPreimage := LocalContextPreimage(pre, post)
			require.Equal(t, test.expected, actualPreimage)
			localContext := CreateLocalContext(pre, post)
			require.Equal(t, crypto.Keccak256Hash(test.expected), localContext)
		})
	}
}
