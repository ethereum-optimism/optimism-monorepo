package interop

import (
	"fmt"
	"math/big"
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

func setupTwoChains() (*staticConfigSource, *eth.SuperV1, stubTasks) {
	rollupCfg1 := chaincfg.OPSepolia()
	chainCfg1 := chainconfig.OPSepoliaChainConfig()

	rollupCfg2 := *chaincfg.OPSepolia()
	rollupCfg2.L2ChainID = new(big.Int).SetUint64(42)
	chainCfg2 := *chainconfig.OPSepoliaChainConfig()
	chainCfg2.ChainID = rollupCfg2.L2ChainID

	agreedSuperRoot := &eth.SuperV1{
		Timestamp: rollupCfg1.Genesis.L2Time + 1234,
		Chains: []eth.ChainIDAndOutput{
			{ChainID: rollupCfg1.L2ChainID.Uint64(), Output: eth.OutputRoot(&eth.OutputV0{BlockHash: common.Hash{0x11}})},
			{ChainID: rollupCfg2.L2ChainID.Uint64(), Output: eth.OutputRoot(&eth.OutputV0{BlockHash: common.Hash{0x22}})},
		},
	}
	configSource := &staticConfigSource{
		rollupCfgs:   []*rollup.Config{rollupCfg1, &rollupCfg2},
		chainConfigs: []*params.ChainConfig{chainCfg1, &chainCfg2},
	}
	tasksStub := stubTasks{
		l2SafeHead: eth.L2BlockRef{Number: 918429823450218}, // Past the claimed block
		blockHash:  common.Hash{0x22},
		outputRoot: eth.Bytes32{0x66},
	}
	return configSource, agreedSuperRoot, tasksStub
}

func TestDeriveBlockForFirstChainFromSuperchainRoot(t *testing.T) {
	logger := testlog.Logger(t, log.LevelError)
	configSource, agreedSuperRoot, tasksStub := setupTwoChains()

	outputRootHash := common.Hash(eth.SuperRoot(agreedSuperRoot))
	l2PreimageOracle, _ := test.NewStubOracle(t)
	l2PreimageOracle.TransitionStates[outputRootHash] = &types.TransitionState{SuperRoot: agreedSuperRoot.Marshal()}

	expectedIntermediateRoot := &types.TransitionState{
		SuperRoot: agreedSuperRoot.Marshal(),
		PendingProgress: []types.OptimisticBlock{
			{BlockHash: tasksStub.blockHash, OutputRoot: tasksStub.outputRoot},
		},
		Step: 1,
	}

	expectedClaim, err := expectedIntermediateRoot.Hash()
	require.NoError(t, err)

	verifyResult(t, logger, tasksStub, configSource, l2PreimageOracle, agreedSuperRoot, outputRootHash, expectedClaim)
}

func TestDeriveBlockForSecondChainFromTransitionState(t *testing.T) {
	logger := testlog.Logger(t, log.LevelError)
	configSource, agreedSuperRoot, tasksStub := setupTwoChains()
	agreedTransitionState := &types.TransitionState{
		SuperRoot: agreedSuperRoot.Marshal(),
		PendingProgress: []types.OptimisticBlock{
			{BlockHash: common.Hash{0xaa}, OutputRoot: eth.Bytes32{6: 22}},
		},
		Step: 1,
	}
	outputRootHash, err := agreedTransitionState.Hash()
	require.NoError(t, err)
	l2PreimageOracle, _ := test.NewStubOracle(t)
	l2PreimageOracle.TransitionStates[outputRootHash] = agreedTransitionState
	expectedIntermediateRoot := &types.TransitionState{
		SuperRoot: agreedSuperRoot.Marshal(),
		PendingProgress: []types.OptimisticBlock{
			{BlockHash: common.Hash{0xaa}, OutputRoot: eth.Bytes32{6: 22}},
			{BlockHash: tasksStub.blockHash, OutputRoot: tasksStub.outputRoot},
		},
		Step: 2,
	}

	expectedClaim, err := expectedIntermediateRoot.Hash()
	require.NoError(t, err)
	verifyResult(t, logger, tasksStub, configSource, l2PreimageOracle, agreedSuperRoot, outputRootHash, expectedClaim)
}

func TestNoOpStep(t *testing.T) {
	logger := testlog.Logger(t, log.LevelError)
	configSource, agreedSuperRoot, tasksStub := setupTwoChains()
	agreedTransitionState := &types.TransitionState{
		SuperRoot: agreedSuperRoot.Marshal(),
		PendingProgress: []types.OptimisticBlock{
			{BlockHash: common.Hash{0xaa}, OutputRoot: eth.Bytes32{6: 22}},
			{BlockHash: tasksStub.blockHash, OutputRoot: tasksStub.outputRoot},
		},
		Step: 2,
	}
	outputRootHash, err := agreedTransitionState.Hash()
	require.NoError(t, err)
	l2PreimageOracle, _ := test.NewStubOracle(t)
	l2PreimageOracle.TransitionStates[outputRootHash] = agreedTransitionState
	expectedIntermediateRoot := *agreedTransitionState // Copy agreed state
	expectedIntermediateRoot.Step = 3

	expectedClaim, err := expectedIntermediateRoot.Hash()
	require.NoError(t, err)
	verifyResult(t, logger, tasksStub, configSource, l2PreimageOracle, agreedSuperRoot, outputRootHash, expectedClaim)
}

func verifyResult(t *testing.T, logger log.Logger, tasks stubTasks, configSource *staticConfigSource, l2PreimageOracle *test.StubBlockOracle, agreedSuperRoot *eth.SuperV1, agreedPrestate common.Hash, expectedClaim common.Hash) {
	bootInfo := &boot.BootInfoInterop{
		AgreedPrestate: agreedPrestate,
		ClaimTimestamp: agreedSuperRoot.Timestamp + 1,
		Claim:          expectedClaim,
		Configs:        configSource,
	}
	err := runInteropProgram(logger, bootInfo, nil, l2PreimageOracle, true, &tasks)
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
