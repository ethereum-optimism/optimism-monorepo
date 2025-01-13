package interop

import (
	"fmt"
	"testing"

	"github.com/ethereum-optimism/optimism/op-node/chaincfg"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-program/chainconfig"
	"github.com/ethereum-optimism/optimism/op-program/client/boot"
	"github.com/ethereum-optimism/optimism/op-program/client/interop/types"
	"github.com/ethereum-optimism/optimism/op-program/client/l1"
	"github.com/ethereum-optimism/optimism/op-program/client/l2"
	"github.com/ethereum-optimism/optimism/op-program/client/l2/test"
	"github.com/ethereum-optimism/optimism/op-program/client/tasks"
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
	chainCfg := chainconfig.OPSepoliaChainConfig()
	chain1Output := &eth.OutputV0{}
	agreedSuperRoot := &eth.SuperV1{
		Timestamp: rollupCfg.Genesis.L2Time + 1234,
		Chains:    []eth.ChainIDAndOutput{{ChainID: rollupCfg.L2ChainID.Uint64(), Output: eth.OutputRoot(chain1Output)}},
	}
	outputRootHash := common.Hash(eth.SuperRoot(agreedSuperRoot))
	l2PreimageOracle, _ := test.NewStubOracle(t)
	l2PreimageOracle.TransitionStates[outputRootHash] = &types.TransitionState{SuperRoot: agreedSuperRoot.Marshal()}
	tasks := stubTasks{
		l2SafeHead: eth.L2BlockRef{
			Number: 56,
			Hash:   common.Hash{0x11},
		},
		blockHash:  common.Hash{0x22},
		outputRoot: eth.Bytes32{0x66},
	}
	expectedIntermediateRoot := &types.TransitionState{
		SuperRoot: agreedSuperRoot.Marshal(),
		PendingProgress: []types.OptimisticBlock{
			{BlockHash: tasks.blockHash, OutputRoot: tasks.outputRoot},
		},
		Step: 1,
	}

	expectedClaim, err := expectedIntermediateRoot.Hash()
	require.NoError(t, err)
	bootInfo := &boot.BootInfoInterop{
		AgreedPrestate: outputRootHash,
		ClaimTimestamp: agreedSuperRoot.Timestamp + 1,
		Claim:          expectedClaim,
		Configs: &staticConfigSource{
			rollupCfgs:   []*rollup.Config{rollupCfg},
			chainConfigs: []*params.ChainConfig{chainCfg},
		},
	}
	err = runInteropProgram(logger, bootInfo, nil, l2PreimageOracle, true, &tasks)
	require.NoError(t, err)
}

type stubTasks struct {
	l2SafeHead eth.L2BlockRef
	blockHash  common.Hash
	outputRoot eth.Bytes32
	err        error
}

func (t *stubTasks) RunDerivation(
	_ log.Logger,
	_ *rollup.Config,
	_ *params.ChainConfig,
	_ common.Hash,
	_ eth.Bytes32,
	_ uint64,
	_ l1.Oracle,
	_ l2.Oracle) (tasks.DerivationResult, error) {
	return tasks.DerivationResult{
		Head:       t.l2SafeHead,
		BlockHash:  t.blockHash,
		OutputRoot: t.outputRoot,
	}, t.err
}

type staticConfigSource struct {
	rollupCfgs   []*rollup.Config
	chainConfigs []*params.ChainConfig
}

func (s *staticConfigSource) RollupConfig(chainID uint64) (*rollup.Config, error) {
	for _, cfg := range s.rollupCfgs {
		if cfg.L2ChainID.Uint64() == chainID {
			return cfg, nil
		}
	}
	return nil, fmt.Errorf("no rollup config found for chain %d", chainID)
}

func (s *staticConfigSource) ChainConfig(chainID uint64) (*params.ChainConfig, error) {
	for _, cfg := range s.chainConfigs {
		if cfg.ChainID.Uint64() == chainID {
			return cfg, nil
		}
	}
	return nil, fmt.Errorf("no chain config found for chain %d", chainID)
}
