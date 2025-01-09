package interop

import (
	"testing"

	"github.com/ethereum-optimism/optimism/op-node/chaincfg"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-program/client/boot"
	"github.com/ethereum-optimism/optimism/op-program/client/claim"
	"github.com/ethereum-optimism/optimism/op-program/client/interop/types"
	"github.com/ethereum-optimism/optimism/op-program/client/l1"
	"github.com/ethereum-optimism/optimism/op-program/client/l2"
	"github.com/ethereum-optimism/optimism/op-program/client/l2/test"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/stretchr/testify/require"
)

func TestDeriveBlockForFirstChainFromSuperchainRoot(t *testing.T) {
	logger := testlog.Logger(t, log.LevelError)
	rollupCfg := chaincfg.OPSepolia()
	chain1Output := &eth.OutputV0{}
	agreedSuperRoot := &eth.SuperV1{
		Timestamp: rollupCfg.Genesis.L2Time + 1234,
		Outputs:   []eth.Bytes32{eth.OutputRoot(chain1Output)},
	}
	outputRootHash := common.Hash(eth.SuperRoot(agreedSuperRoot))
	bootInfo := &boot.BootInfo{
		L2OutputRoot: outputRootHash,
		RollupConfig: rollupCfg,
	}
	l2PreimageOracle, _ := test.NewStubOracle(t)
	l2PreimageOracle.TransitionStates[outputRootHash] = &types.TransitionState{SuperRoot: agreedSuperRoot.Marshal()}
	tasks := stubTasks{
		l2BlockRef: eth.L2BlockRef{
			Number: 56,
			Hash:   common.Hash{0x11},
		},
		outputRoot: eth.Bytes32{0x66},
	}
	//expectedIntermediateRoot := &
	err := runInteropProgram(logger, bootInfo, nil, l2PreimageOracle, &tasks)
	require.ErrorIs(t, err, claim.ErrClaimNotValid)
}

type stubTasks struct {
	l2BlockRef eth.L2BlockRef
	outputRoot eth.Bytes32
	err        error
}

func (t *stubTasks) RunDerivation(
	logger log.Logger,
	rollupCfg *rollup.Config,
	l2ChainConfig *params.ChainConfig,
	l1Head common.Hash,
	agreedOutputRoot eth.Bytes32,
	claimedBlockNumber uint64,
	l1Oracle l1.Oracle,
	l2Oracle l2.Oracle) (eth.L2BlockRef, eth.Bytes32, error) {
	return t.l2BlockRef, t.outputRoot, t.err
}
