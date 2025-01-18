package opcm

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/lmittmann/w3"
)

var anchorRootFunc = w3.MustNewFunc(`
dummy((bytes32 root, uint256 l2BlockNumber) outputRoot)
`, "")

type StartingAnchorRoot struct {
	Root          common.Hash
	L2BlockNumber *big.Int
}

var DefaultStartingAnchorRoot = StartingAnchorRoot{
	Root:          common.Hash{0xde, 0xad},
	L2BlockNumber: common.Big0,
}

func EncodeStartingAnchorRoot(root StartingAnchorRoot) ([]byte, error) {
	encoded, err := anchorRootFunc.EncodeArgs(root)
	if err != nil {
		return nil, fmt.Errorf("error encoding anchor root: %w", err)
	}
	// Chop off the function selector since w3 can't serialize structs directly
	return encoded[4:], nil
}
