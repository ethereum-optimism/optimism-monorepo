package opcm

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/lmittmann/w3"
)

var anchorRootFunc = w3.MustNewFunc(`
dummy((uint32 gameType, (bytes32 root, uint256 l2BlockNumber) outputRoot)[] roots)
`, "")

type StartingAnchorRoot struct {
	GameType      uint32
	Root          common.Hash
	L2BlockNumber *big.Int
}

var DefaultStartingAnchorRoot = StartingAnchorRoot{
	GameType:      1,
	Root:          common.Hash{0xde, 0xad},
	L2BlockNumber: common.Big0,
}

type encodingStartingAnchorRoot struct {
	GameType   uint32
	OutputRoot struct {
		Root          common.Hash
		L2BlockNumber *big.Int
	}
}

func EncodeStartingAnchorRoots(roots []StartingAnchorRoot) ([]byte, error) {
	args := make([]encodingStartingAnchorRoot, len(roots))
	for i, root := range roots {
		args[i] = encodingStartingAnchorRoot{
			GameType: root.GameType,
			OutputRoot: struct {
				Root          common.Hash
				L2BlockNumber *big.Int
			}{
				Root:          root.Root,
				L2BlockNumber: root.L2BlockNumber,
			},
		}
	}
	encoded, err := anchorRootFunc.EncodeArgs(args)
	if err != nil {
		return nil, fmt.Errorf("error encoding anchor roots: %w", err)
	}
	// Chop off the function selector since w3 can't serialize structs directly
	return encoded[4:], nil
}
