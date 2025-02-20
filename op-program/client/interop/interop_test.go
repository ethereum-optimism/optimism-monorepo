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
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
	supervisortypes "github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
	"github.com/ethereum/go-ethereum/common"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/types/interoptypes"
	"github.com/ethereum/go-ethereum/crypto"
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
	depset, _ := depset.NewStaticConfigDependencySet(map[eth.ChainID]*depset.StaticConfigDependency{
		eth.ChainIDFromBig(rollupCfg1.L2ChainID): {ChainIndex: chainA, ActivationTime: 0, HistoryMinTime: 0},
		eth.ChainIDFromBig(rollupCfg2.L2ChainID): {ChainIndex: chainB, ActivationTime: 0, HistoryMinTime: 0},
	})
	configSource := &staticConfigSource{
		rollupCfgs:   []*rollup.Config{rollupCfg1, &rollupCfg2},
		chainConfigs: []*params.ChainConfig{chainCfg1, &chainCfg2},
		depset:       depset,
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

var (
	initiatingMessageTopic   = crypto.Keccak256Hash([]byte("Test()"))
	initPayloadHash          = crypto.Keccak256Hash(initiatingMessageTopic[:])
	initiatingMessageOrigin  = common.Address{0xaa}
	initiatingMessageOrigin2 = common.Address{0xbb}
)

const (
	chainA supervisortypes.ChainIndex = 0
	chainB supervisortypes.ChainIndex = 1
)

func TestDeriveBlockForConsolidateStep(t *testing.T) {
	createExecMessage := func(initIncludedIn uint64, config *staticConfigSource) interoptypes.Message {
		exec := interoptypes.Message{
			Identifier: interoptypes.Identifier{
				Origin:      initiatingMessageOrigin,
				BlockNumber: initIncludedIn,
				LogIndex:    0,
				Timestamp:   initIncludedIn * config.rollupCfgs[chainA].BlockTime,
				ChainID:     uint256.Int(eth.ChainIDFromBig(config.rollupCfgs[chainA].L2ChainID)),
			},
			PayloadHash: initPayloadHash,
		}
		return exec
	}
	createInitLog := func() *gethTypes.Log {
		return &gethTypes.Log{
			Address: initiatingMessageOrigin,
			Topics:  []common.Hash{initiatingMessageTopic},
		}
	}

	cases := []struct {
		name     string
		testCase consolidationTestCase
	}{
		{
			name:     "HappyPath",
			testCase: consolidationTestCase{},
		},
		{
			name: "HappyPathWithValidMessages-ExecOnChainB",
			testCase: consolidationTestCase{
				logBuilderFn: func(includeBlockNumbers map[supervisortypes.ChainIndex]uint64, config *staticConfigSource) map[supervisortypes.ChainIndex][]*gethTypes.Log {
					init := createInitLog()
					exec := createExecMessage(includeBlockNumbers[chainA], config)
					return map[supervisortypes.ChainIndex][]*gethTypes.Log{chainA: {init}, chainB: {convertExecutingMessageToLog(t, exec)}}
				},
			},
		},
		{
			name: "HappyPathWithValidMessages-ExecOnChainA",
			testCase: consolidationTestCase{
				logBuilderFn: func(includeBlockNumbers map[supervisortypes.ChainIndex]uint64, config *staticConfigSource) map[supervisortypes.ChainIndex][]*gethTypes.Log {
					init := createInitLog()
					initPayloadHash := crypto.Keccak256Hash(initiatingMessageTopic[:])
					execMsg := interoptypes.Message{
						Identifier: interoptypes.Identifier{
							Origin:      init.Address,
							BlockNumber: includeBlockNumbers[chainB],
							LogIndex:    0,
							Timestamp:   includeBlockNumbers[chainB] * config.rollupCfgs[chainB].BlockTime,
							ChainID:     uint256.Int(eth.ChainIDFromBig(config.rollupCfgs[chainB].L2ChainID)),
						},
						PayloadHash: initPayloadHash,
					}
					exec := convertExecutingMessageToLog(t, execMsg)
					return map[supervisortypes.ChainIndex][]*gethTypes.Log{chainA: {exec}, chainB: {init}}
				},
			},
		},
		{
			name: "HappyPathWithValidMessages-ExecOnChainB-NonZeroLogIndex",
			testCase: consolidationTestCase{
				logBuilderFn: func(includeBlockNumbers map[supervisortypes.ChainIndex]uint64, config *staticConfigSource) map[supervisortypes.ChainIndex][]*gethTypes.Log {
					init1 := &gethTypes.Log{
						Address: initiatingMessageOrigin,
						Topics:  []common.Hash{initiatingMessageTopic},
					}
					init2 := &gethTypes.Log{
						Address: initiatingMessageOrigin2,
						Topics:  []common.Hash{initiatingMessageTopic},
					}
					exec := createExecMessage(includeBlockNumbers[chainA], config)
					exec.Identifier.Origin = init2.Address
					exec.Identifier.LogIndex = 1
					return map[supervisortypes.ChainIndex][]*gethTypes.Log{
						chainA: {init1, init2},
						chainB: {convertExecutingMessageToLog(t, exec)},
					}
				},
			},
		},
		{
			name: "ReplaceChainB-UnknownChainID",
			testCase: consolidationTestCase{
				logBuilderFn: func(includeBlockNumbers map[supervisortypes.ChainIndex]uint64, config *staticConfigSource) map[supervisortypes.ChainIndex][]*gethTypes.Log {
					init := createInitLog()
					exec := createExecMessage(includeBlockNumbers[chainA], config)
					exec.Identifier.ChainID = uint256.Int(eth.ChainIDFromUInt64(0xdeadbeef))
					return map[supervisortypes.ChainIndex][]*gethTypes.Log{chainA: {init}, chainB: {convertExecutingMessageToLog(t, exec)}}
				},
				expectBlockReplacements: func(config *staticConfigSource) []supervisortypes.ChainIndex {
					return []supervisortypes.ChainIndex{chainB}
				},
			},
		},
		{
			name: "ReplaceChainB-InvalidLogIndex",
			testCase: consolidationTestCase{
				logBuilderFn: func(includeBlockNumbers map[supervisortypes.ChainIndex]uint64, config *staticConfigSource) map[supervisortypes.ChainIndex][]*gethTypes.Log {
					init1 := &gethTypes.Log{
						Address: initiatingMessageOrigin,
						Topics:  []common.Hash{initiatingMessageTopic},
					}
					init2 := &gethTypes.Log{
						Address: initiatingMessageOrigin2,
						Topics:  []common.Hash{initiatingMessageTopic},
					}
					exec := createExecMessage(includeBlockNumbers[chainA], config)
					exec.Identifier.Origin = init2.Address
					exec.Identifier.LogIndex = 0
					return map[supervisortypes.ChainIndex][]*gethTypes.Log{
						chainA: {init1, init2},
						chainB: {convertExecutingMessageToLog(t, exec)},
					}
				},
				expectBlockReplacements: func(config *staticConfigSource) []supervisortypes.ChainIndex {
					return []supervisortypes.ChainIndex{chainB}
				},
			},
		},
		{
			name: "ReplaceChainB-InvalidPayloadHash",
			testCase: consolidationTestCase{
				logBuilderFn: func(includeBlockNumbers map[supervisortypes.ChainIndex]uint64, config *staticConfigSource) map[supervisortypes.ChainIndex][]*gethTypes.Log {
					init := createInitLog()
					execMsg := createExecMessage(includeBlockNumbers[chainA], config)
					execMsg.PayloadHash = crypto.Keccak256Hash([]byte("invalid hash"))
					return map[supervisortypes.ChainIndex][]*gethTypes.Log{chainA: {init}, chainB: {convertExecutingMessageToLog(t, execMsg)}}
				},
				expectBlockReplacements: func(config *staticConfigSource) []supervisortypes.ChainIndex {
					return []supervisortypes.ChainIndex{chainB}
				},
			},
		},
		{
			name: "ReplaceChainB-InvalidTimestamp",
			testCase: consolidationTestCase{
				logBuilderFn: func(includeBlockNumbers map[supervisortypes.ChainIndex]uint64, config *staticConfigSource) map[supervisortypes.ChainIndex][]*gethTypes.Log {
					init := createInitLog()
					execMsg := createExecMessage(includeBlockNumbers[chainA], config)
					execMsg.Identifier.Timestamp = execMsg.Identifier.Timestamp - 1
					return map[supervisortypes.ChainIndex][]*gethTypes.Log{chainA: {init}, chainB: {convertExecutingMessageToLog(t, execMsg)}}
				},
				expectBlockReplacements: func(config *staticConfigSource) []supervisortypes.ChainIndex {
					return []supervisortypes.ChainIndex{chainB}
				},
			},
		},
		{
			name: "ReplaceBothChains",
			testCase: consolidationTestCase{
				logBuilderFn: func(includeBlockNumbers map[supervisortypes.ChainIndex]uint64, config *staticConfigSource) map[supervisortypes.ChainIndex][]*gethTypes.Log {
					invalidExecMsg := createExecMessage(includeBlockNumbers[chainA], config)
					invalidExecMsg.PayloadHash = crypto.Keccak256Hash([]byte("invalid hash"))
					log := convertExecutingMessageToLog(t, invalidExecMsg)
					return map[supervisortypes.ChainIndex][]*gethTypes.Log{chainA: {log}, chainB: {log}}
				},
				expectBlockReplacements: func(config *staticConfigSource) []supervisortypes.ChainIndex {
					return []supervisortypes.ChainIndex{chainA, chainB}
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

// expectBlockReplacementsFn returns the chain indexes containing an optimistic block that must be replaced
type expectBlockReplacementsFn func(config *staticConfigSource) (chainIndexesToReplace []supervisortypes.ChainIndex)

type logBuilderFn func(includeBlockNumbers map[supervisortypes.ChainIndex]uint64, config *staticConfigSource) map[supervisortypes.ChainIndex][]*gethTypes.Log

type consolidationTestCase struct {
	expectBlockReplacements expectBlockReplacementsFn
	logBuilderFn            logBuilderFn
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

	var logA, logB []*gethTypes.Log
	if testCase.logBuilderFn != nil {
		logs := testCase.logBuilderFn(
			map[supervisortypes.ChainIndex]uint64{0: block1A.NumberU64() + 1, 1: block1B.NumberU64() + 1},
			configSource,
		)
		logA = logs[chainA]
		logB = logs[chainB]
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

func convertExecutingMessageToLog(t *testing.T, msg interoptypes.Message) *gethTypes.Log {
	id := msg.Identifier
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
	return &gethTypes.Log{
		Address: params.InteropCrossL2InboxAddress,
		Topics:  []common.Hash{interoptypes.ExecutingMessageEventTopic, msg.PayloadHash},
		Data:    data,
	}
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
	depset       *depset.StaticConfigDependencySet
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

func (s *staticConfigSource) DependencySet(chainID eth.ChainID) (depset.DependencySet, error) {
	return s.depset, nil
}
