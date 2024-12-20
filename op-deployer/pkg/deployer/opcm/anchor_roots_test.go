package opcm

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestEncodeStartingAnchorRoots(t *testing.T) {
	encoded, err := EncodeStartingAnchorRoots([]StartingAnchorRoot{
		DefaultStartingAnchorRoot,
	})
	require.NoError(t, err)
	require.EqualValues(t, PermissionedGameStartingAnchorRoots, encoded)

	encoded, err = EncodeStartingAnchorRoots([]StartingAnchorRoot{
		{
			GameType:      0,
			L2BlockNumber: common.Big0,
		},
		{
			GameType:      1,
			Root:          common.Hash{0xde, 0xad},
			L2BlockNumber: big.NewInt(0),
		},
	})
	require.NoError(t, err)
	require.EqualValues(t,
		common.Hex2Bytes(
			"0000000000000000000000000000000000000000000000000000000000000020"+
				"0000000000000000000000000000000000000000000000000000000000000002"+
				"0000000000000000000000000000000000000000000000000000000000000000"+
				"0000000000000000000000000000000000000000000000000000000000000000"+
				"0000000000000000000000000000000000000000000000000000000000000000"+
				"0000000000000000000000000000000000000000000000000000000000000001"+
				"dead000000000000000000000000000000000000000000000000000000000000"+
				"0000000000000000000000000000000000000000000000000000000000000000"),
		encoded,
	)
}
