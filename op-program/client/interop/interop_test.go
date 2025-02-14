package interop

import (
	"encoding/binary"
	"fmt"
	"math/big"
	"math/rand"
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
	"github.com/ethereum-optimism/optimism/op-service/testutils"
	"github.com/ethereum/go-ethereum/common"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/types/interoptypes"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func setupTwoChains() (*staticConfigSource, *eth.SuperV1, *stubTasks) {
	rollupCfg1 := chaincfg.OPSepolia()
	chainCfg1 := chainconfig.OPSepoliaChainConfig()

	rollupCfg2 := *chaincfg.OPSepolia()
	rollupCfg2.L2ChainID = new(big.Int).SetUint64(42)
	chainCfg2 := *chainconfig.OPSepoliaChainConfig()
	chainCfg2.ChainID = rollupCfg2.L2ChainID

	agreedSuperRoot := &eth.SuperV1{
		Timestamp: rollupCfg1.Genesis.L2Time + 1234,
		Chains: []eth.ChainIDAndOutput{
			{ChainID: eth.ChainIDFromBig(rollupCfg1.L2ChainID), Output: eth.OutputRoot(&eth.OutputV0{BlockHash: common.Hash{0x11}})},
			{ChainID: eth.ChainIDFromBig(rollupCfg2.L2ChainID), Output: eth.OutputRoot(&eth.OutputV0{BlockHash: common.Hash{0x22}})},
		},
	}
	configSource := &staticConfigSource{
		rollupCfgs:   []*rollup.Config{rollupCfg1, &rollupCfg2},
		chainConfigs: []*params.ChainConfig{chainCfg1, &chainCfg2},
	}
	tasksStub := &stubTasks{
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

	expectedClaim := expectedIntermediateRoot.Hash()
	verifyResult(t, logger, tasksStub, configSource, l2PreimageOracle, outputRootHash, agreedSuperRoot.Timestamp+100000, expectedClaim)
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
	outputRootHash := agreedTransitionState.Hash()
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

	expectedClaim := expectedIntermediateRoot.Hash()
	verifyResult(t, logger, tasksStub, configSource, l2PreimageOracle, outputRootHash, agreedSuperRoot.Timestamp+100000, expectedClaim)
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
	outputRootHash := agreedTransitionState.Hash()
	l2PreimageOracle, _ := test.NewStubOracle(t)
	l2PreimageOracle.TransitionStates[outputRootHash] = agreedTransitionState
	expectedIntermediateRoot := *agreedTransitionState // Copy agreed state
	expectedIntermediateRoot.Step = 3

	expectedClaim := expectedIntermediateRoot.Hash()
	verifyResult(t, logger, tasksStub, configSource, l2PreimageOracle, outputRootHash, agreedSuperRoot.Timestamp+100000, expectedClaim)
}

func TestDeriveBlockForConsolidateStep(t *testing.T) {
	cases := []struct {
		name     string
		testCase consolidationTestCase
	}{
		{
			name:     "HappyPath",
			testCase: consolidationTestCase{},
		},
		{
			name: "HappyPathWithValidMessages",
			testCase: consolidationTestCase{
				stubExecMsgFn: func(includeChainIndex int, includeBlockNum uint64, config *staticConfigSource) []executingMessage {
					if includeChainIndex == 0 {
						return nil
					} else {
						return []executingMessage{
							{
								ChainID:   eth.ChainIDFromBig(config.rollupCfgs[0].L2ChainID),
								BlockNum:  includeBlockNum,
								LogIdx:    0,
								Timestamp: includeBlockNum * config.rollupCfgs[1].BlockTime,
							},
						}
					}
				},
			},
		},
		{
			name: "DepositsOnlyBlockReplacement-ChainA",
			// Mock block2A (chain A, block 2) replaced with a deposit-only block
			// Due to a self-referential invalid executing message.
			testCase: consolidationTestCase{
				stubExecMsgFn: func(includeChainIndex int, includeBlockNum uint64, config *staticConfigSource) []executingMessage {
					if includeChainIndex == 0 {
						return []executingMessage{
							{
								ChainID:   eth.ChainIDFromBig(config.rollupCfgs[0].L2ChainID),
								BlockNum:  includeBlockNum,
								LogIdx:    0,
								Timestamp: includeBlockNum * config.rollupCfgs[0].BlockTime,
							},
						}
					} else {
						return nil
					}
				},
				expectBlockReplacements: func(config *staticConfigSource) []uint64 {
					return []uint64{0}
				},
			},
		},
		{
			name: "DepositsOnlyBlockReplacement-ChainB",
			// Mock block2B (chain B, block 2) replaced with a deposit-only block
			// Due to a self-referential invalid executing message.
			testCase: consolidationTestCase{
				stubExecMsgFn: func(includeChainIndex int, includeBlockNum uint64, config *staticConfigSource) []executingMessage {
					if includeChainIndex == 0 {
						return nil
					} else {
						return []executingMessage{
							{
								ChainID:   eth.ChainIDFromBig(config.rollupCfgs[1].L2ChainID),
								BlockNum:  includeBlockNum,
								LogIdx:    0,
								Timestamp: includeBlockNum * config.rollupCfgs[1].BlockTime,
							},
						}
					}
				},
				expectBlockReplacements: func(config *staticConfigSource) []uint64 {
					return []uint64{1}
				},
			},
		},
		{
			name: "DepositsOnlyBlockReplacement-BothChains",
			testCase: consolidationTestCase{
				stubExecMsgFn: func(includeChainIndex int, includeBlockNum uint64, config *staticConfigSource) []executingMessage {
					return []executingMessage{
						{
							ChainID:   eth.ChainIDFromBig(config.rollupCfgs[includeChainIndex].L2ChainID),
							BlockNum:  includeBlockNum,
							LogIdx:    0,
							Timestamp: includeBlockNum * config.rollupCfgs[includeChainIndex].BlockTime,
						},
					}
				},
				expectBlockReplacements: func(config *staticConfigSource) []uint64 {
					return []uint64{0, 1}
				},
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			runConsolidationTestCase(t, tt.testCase)
		})
	}
}

// stubExecMsgFn returns the executing messages to stub inclusion for the specified block
type stubExecMsgFn func(includeChainIndex int, includeBlockNum uint64, config *staticConfigSource) []executingMessage

// expectBlockReplacementsFn returns the chain indexes containing an optimistic block that must be replaced
type expectBlockReplacementsFn func(config *staticConfigSource) (chainIDsToReplace []uint64)

type consolidationTestCase struct {
	stubExecMsgFn           stubExecMsgFn
	expectBlockReplacements expectBlockReplacementsFn
}

func runConsolidationTestCase(t *testing.T, testCase consolidationTestCase) {
	logger := testlog.Logger(t, log.LevelError)
	configSource, agreedSuperRoot, tasksStub := setupTwoChains()
	defer tasksStub.AssertExpectations(t)
	rng := rand.New(rand.NewSource(123))

	configA := configSource.rollupCfgs[0]
	configB := configSource.rollupCfgs[1]

	block1A, _ := createBlock(rng, configA, 1, nil)
	block1B, _ := createBlock(rng, configB, 1, nil)

	var logA []*gethTypes.Log
	if testCase.stubExecMsgFn != nil {
		execMsgA := testCase.stubExecMsgFn(0, block1A.NumberU64()+1, configSource)
		logA = convertExecutingMessagesToLog(t, execMsgA)
	}
	var logB []*gethTypes.Log
	if testCase.stubExecMsgFn != nil {
		execMsgB := testCase.stubExecMsgFn(1, block1B.NumberU64()+1, configSource)
		logB = convertExecutingMessagesToLog(t, execMsgB)
	}
	block2A, block2AReceipts := createBlock(rng, configA, 2, gethTypes.Receipts{{Logs: logA}})
	block2B, block2BReceipts := createBlock(rng, configB, 2, gethTypes.Receipts{{Logs: logB}})

	pendingOutputs := [2]*eth.OutputV0{
		0: createOutput(block2A.Hash()),
		1: createOutput(block2B.Hash()),
	}
	finalTransitionState := &types.TransitionState{
		SuperRoot: agreedSuperRoot.Marshal(),
		PendingProgress: []types.OptimisticBlock{
			{BlockHash: block2A.Hash(), OutputRoot: eth.OutputRoot(pendingOutputs[0])},
			{BlockHash: block2B.Hash(), OutputRoot: eth.OutputRoot(pendingOutputs[1])},
		},
		Step: ConsolidateStep,
	}
	outputRootHash := finalTransitionState.Hash()
	l2PreimageOracle, _ := test.NewStubOracle(t)
	l2PreimageOracle.TransitionStates[outputRootHash] = finalTransitionState

	l2PreimageOracle.Outputs[common.Hash(agreedSuperRoot.Chains[0].Output)] = createOutput(block1A.Hash())
	l2PreimageOracle.Outputs[common.Hash(agreedSuperRoot.Chains[1].Output)] = createOutput(block1B.Hash())
	l2PreimageOracle.BlockData = map[common.Hash]*gethTypes.Block{
		block2A.Hash(): block2A,
		block2B.Hash(): block2B,
	}
	l2PreimageOracle.Blocks[block1A.Hash()] = block1A
	l2PreimageOracle.Blocks[block2A.Hash()] = block2A
	l2PreimageOracle.Blocks[block2B.Hash()] = block2B

	l2PreimageOracle.Receipts[block2A.Hash()] = block2AReceipts
	l2PreimageOracle.Receipts[block2B.Hash()] = block2BReceipts

	finalRoots := [2]eth.Bytes32{
		finalTransitionState.PendingProgress[0].OutputRoot,
		finalTransitionState.PendingProgress[1].OutputRoot,
	}
	if testCase.expectBlockReplacements != nil {
		for _, chainIndexToReplace := range testCase.expectBlockReplacements(configSource) {
			// stub output root preimage of the replaced block
			replacedBlockOutput := pendingOutputs[chainIndexToReplace]
			replacedBlockOutputRoot := common.Hash(eth.OutputRoot(replacedBlockOutput))
			l2PreimageOracle.Outputs[replacedBlockOutputRoot] = replacedBlockOutput

			depositsOnlyBlock, _ := createBlock(rng, configSource.rollupCfgs[chainIndexToReplace], 2, nil)
			depositsOnlyOutputRoot := eth.OutputRoot(createOutput(depositsOnlyBlock.Hash()))
			tasksStub.ExpectBuildDepositOnlyBlock(common.Hash{}, agreedSuperRoot.Chains[chainIndexToReplace].Output, depositsOnlyBlock.Hash(), depositsOnlyOutputRoot)
			finalRoots[chainIndexToReplace] = depositsOnlyOutputRoot
		}
	}
	expectedClaim := common.Hash(eth.SuperRoot(&eth.SuperV1{
		Timestamp: agreedSuperRoot.Timestamp + 1,
		Chains: []eth.ChainIDAndOutput{
			{
				ChainID: eth.ChainIDFromBig(configA.L2ChainID),
				Output:  finalRoots[0],
			},
			{
				ChainID: eth.ChainIDFromBig(configB.L2ChainID),
				Output:  finalRoots[1],
			},
		},
	}))

	verifyResult(
		t,
		logger,
		tasksStub,
		configSource,
		l2PreimageOracle,
		outputRootHash,
		agreedSuperRoot.Timestamp+100000,
		expectedClaim,
	)
}

func createOutput(blockHash common.Hash) *eth.OutputV0 {
	return &eth.OutputV0{BlockHash: blockHash}
}

type executingMessage struct {
	ChainID   eth.ChainID
	BlockNum  uint64
	LogIdx    uint32
	Timestamp uint64
}

func convertExecutingMessagesToLog(t *testing.T, msgs []executingMessage) []*gethTypes.Log {
	logs := make([]*gethTypes.Log, 0, len(msgs))
	for _, msg := range msgs {
		id := interoptypes.Identifier{
			Origin:      common.Address{0xaa},
			BlockNumber: msg.BlockNum,
			LogIndex:    msg.LogIdx,
			Timestamp:   msg.Timestamp,
			ChainID:     uint256.Int(msg.ChainID),
		}
		data := make([]byte, 0, 32*5)
		data = append(data, make([]byte, 12)...)
		data = append(data, id.Origin.Bytes()...)
		data = append(data, make([]byte, 32-8)...)
		data = append(data, binary.BigEndian.AppendUint64(nil, id.BlockNumber)...)
		data = append(data, make([]byte, 32-4)...)
		data = append(data, binary.BigEndian.AppendUint32(nil, id.LogIndex)...)
		data = append(data, make([]byte, 32-8)...)
		data = append(data, binary.BigEndian.AppendUint64(nil, id.Timestamp)...)
		b := id.ChainID.Bytes32()
		data = append(data, b[:]...)
		require.Equal(t, len(data), 32*5)

		payloadHash := common.Hash{0x01, 0x02, 0x03}
		logs = append(logs, &gethTypes.Log{
			Address: params.InteropCrossL2InboxAddress,
			Topics:  []common.Hash{interoptypes.ExecutingMessageEventTopic, payloadHash},
			Data:    data,
		})
	}
	return logs
}

func createBlock(rng *rand.Rand,
	config *rollup.Config,
	blockNum int64, receipts gethTypes.Receipts) (*gethTypes.Block, gethTypes.Receipts) {
	block, randomReceipts := testutils.RandomBlock(rng, 1)
	receipts = append(receipts, randomReceipts...)
	header := block.Header()
	header.Time = uint64(blockNum) * config.BlockTime
	header.Number = big.NewInt(blockNum)
	return gethTypes.NewBlock(
		header,
		block.Body(),
		receipts,
		trie.NewStackTrie(nil),
		gethTypes.DefaultBlockConfig,
	), receipts
}

func TestTraceExtensionOnceClaimedTimestampIsReached(t *testing.T) {
	logger := testlog.Logger(t, log.LevelError)
	configSource, agreedSuperRoot, tasksStub := setupTwoChains()
	agreedPrestatehash := common.Hash(eth.SuperRoot(agreedSuperRoot))
	l2PreimageOracle, _ := test.NewStubOracle(t)
	l2PreimageOracle.TransitionStates[agreedPrestatehash] = &types.TransitionState{SuperRoot: agreedSuperRoot.Marshal()}

	// We have reached the game's timestamp so should just trace extend the agreed claim
	expectedClaim := agreedPrestatehash
	verifyResult(t, logger, tasksStub, configSource, l2PreimageOracle, agreedPrestatehash, agreedSuperRoot.Timestamp, expectedClaim)
}

func TestPanicIfAgreedPrestateIsAfterGameTimestamp(t *testing.T) {
	logger := testlog.Logger(t, log.LevelError)
	configSource, agreedSuperRoot, tasksStub := setupTwoChains()
	agreedPrestatehash := common.Hash(eth.SuperRoot(agreedSuperRoot))
	l2PreimageOracle, _ := test.NewStubOracle(t)
	l2PreimageOracle.TransitionStates[agreedPrestatehash] = &types.TransitionState{SuperRoot: agreedSuperRoot.Marshal()}

	// We have reached the game's timestamp so should just trace extend the agreed claim
	expectedClaim := agreedPrestatehash
	require.PanicsWithError(t, fmt.Sprintf("agreed prestate timestamp %v is after the game timestamp %v", agreedSuperRoot.Timestamp, agreedSuperRoot.Timestamp-1), func() {
		verifyResult(t, logger, tasksStub, configSource, l2PreimageOracle, agreedPrestatehash, agreedSuperRoot.Timestamp-1, expectedClaim)
	})
}

func verifyResult(t *testing.T, logger log.Logger, tasks *stubTasks, configSource *staticConfigSource, l2PreimageOracle *test.StubBlockOracle, agreedPrestate common.Hash, gameTimestamp uint64, expectedClaim common.Hash) {
	bootInfo := &boot.BootInfoInterop{
		AgreedPrestate: agreedPrestate,
		GameTimestamp:  gameTimestamp,
		Claim:          expectedClaim,
		Configs:        configSource,
	}
	err := runInteropProgram(logger, bootInfo, nil, l2PreimageOracle, true, tasks)
	require.NoError(t, err)
}

type stubTasks struct {
	mock.Mock
	l2SafeHead eth.L2BlockRef
	blockHash  common.Hash
	outputRoot eth.Bytes32
	err        error
}

var _ taskExecutor = (*stubTasks)(nil)

func (t *stubTasks) RunDerivation(
	_ log.Logger,
	_ *rollup.Config,
	_ *params.ChainConfig,
	_ common.Hash,
	_ eth.Bytes32,
	_ uint64,
	_ l1.Oracle,
	_ l2.Oracle,
) (tasks.DerivationResult, error) {
	return tasks.DerivationResult{
		Head:       t.l2SafeHead,
		BlockHash:  t.blockHash,
		OutputRoot: t.outputRoot,
	}, t.err
}

func (t *stubTasks) BuildDepositOnlyBlock(
	logger log.Logger,
	rollupCfg *rollup.Config,
	l2ChainConfig *params.ChainConfig,
	l1Head common.Hash,
	agreedL2OutputRoot eth.Bytes32,
	l1Oracle l1.Oracle,
	l2Oracle l2.Oracle,
	optimisticBlock *gethTypes.Block,
) (common.Hash, eth.Bytes32, error) {
	out := t.Mock.Called(
		logger,
		rollupCfg,
		l2ChainConfig,
		l1Head,
		agreedL2OutputRoot,
		l1Oracle,
		l2Oracle,
		optimisticBlock,
	)
	return out.Get(0).(common.Hash), out.Get(1).(eth.Bytes32), nil
}

func (t *stubTasks) ExpectBuildDepositOnlyBlock(
	expectL1Head common.Hash,
	expectAgreedL2OutputRoot eth.Bytes32,
	depositOnlyBlockHash common.Hash,
	depositOnlyOutputRoot eth.Bytes32,
) {
	t.Mock.On(
		"BuildDepositOnlyBlock",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		expectL1Head,
		expectAgreedL2OutputRoot,
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Once().Return(depositOnlyBlockHash, depositOnlyOutputRoot, nil)
}

type staticConfigSource struct {
	rollupCfgs   []*rollup.Config
	chainConfigs []*params.ChainConfig
}

func (s *staticConfigSource) RollupConfig(chainID eth.ChainID) (*rollup.Config, error) {
	for _, cfg := range s.rollupCfgs {
		if eth.ChainIDFromBig(cfg.L2ChainID) == chainID {
			return cfg, nil
		}
	}
	return nil, fmt.Errorf("no rollup config found for chain %d", chainID)
}

func (s *staticConfigSource) ChainConfig(chainID eth.ChainID) (*params.ChainConfig, error) {
	for _, cfg := range s.chainConfigs {
		if eth.ChainIDFromBig(cfg.ChainID) == chainID {
			return cfg, nil
		}
	}
	return nil, fmt.Errorf("no chain config found for chain %d", chainID)
}
