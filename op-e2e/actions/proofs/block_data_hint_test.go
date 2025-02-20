package proofs

import (
	"context"
	"testing"

	altda "github.com/ethereum-optimism/optimism/op-alt-da"
	actionsHelpers "github.com/ethereum-optimism/optimism/op-e2e/actions/helpers"
	"github.com/ethereum-optimism/optimism/op-e2e/actions/proofs/helpers"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils"
	"github.com/ethereum-optimism/optimism/op-node/node/safedb"
	"github.com/ethereum-optimism/optimism/op-node/rollup/event"
	"github.com/ethereum-optimism/optimism/op-node/rollup/sync"
	preimage "github.com/ethereum-optimism/optimism/op-preimage"
	"github.com/ethereum-optimism/optimism/op-program/client/l2"
	"github.com/ethereum-optimism/optimism/op-program/host/kvstore"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/sources"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"
)

func Test_OPProgramAction_BlockDataHint(gt *testing.T) {
	testCfg := &helpers.TestCfg[any]{
		Hardfork: helpers.LatestFork,
	}
	t := actionsHelpers.NewDefaultTesting(gt)
	env := helpers.NewL2FaultProofEnv(t, testCfg, helpers.NewTestParams(), helpers.NewBatcherCfg())

	// Build a block on L2 with 1 tx.
	env.Alice.L2.ActResetTxOpts(t)
	env.Alice.L2.ActSetTxToAddr(&env.Dp.Addresses.Bob)(t)
	env.Alice.L2.ActMakeTx(t)

	env.Sequencer.ActL2StartBlock(t)
	env.Engine.ActL2IncludeTx(env.Alice.Address())(t)
	env.Sequencer.ActL2EndBlock(t)
	env.Alice.L2.ActCheckReceiptStatusOfLastTx(true)(t)

	// Instruct the batcher to submit the block to L1, and include the transaction.
	env.Batcher.ActSubmitAll(t)
	env.Miner.ActL1StartBlock(12)(t)
	env.Miner.ActL1IncludeTxByHash(env.Batcher.LastSubmitted.Hash())(t)
	env.Miner.ActL1EndBlock(t)

	// Finalize the block with the batch on L1.
	env.Miner.ActL1SafeNext(t)
	env.Miner.ActL1FinalizeNext(t)

	// Instruct the sequencer to derive the L2 chain from the data on L1 that the batcher just posted.
	env.Sequencer.ActL1HeadSignal(t)
	env.Sequencer.ActL2PipelineFull(t)

	l1Head := env.Miner.L1Chain().CurrentBlock()
	l2SafeHead := env.Engine.L2Chain().CurrentSafeBlock()

	// Ensure there is only 1 block on L1.
	require.Equal(t, uint64(1), l1Head.Number.Uint64())
	// Ensure the block is marked as safe before we attempt to fault prove it.
	require.Equal(t, uint64(1), l2SafeHead.Number.Uint64())

	// Now create a verifier that syncs up to the safe head parent
	// This simulates a reorg view for the program reexecution
	verifier, verifierEngine := createVerifier(t, env)
	verifier.ActL2EventsUntil(t, func(ev event.Event) bool {
		until := l2SafeHead.Number.Uint64() - 1
		ref, err := verifier.Eng.BlockRefByNumber(context.Background(), until)
		require.NoError(t, err)
		return ref.Number == until
	}, 20, false)
	// Ensure that the block isn't available
	_, err := verifier.Eng.BlockRefByNumber(context.Background(), l2SafeHead.Number.Uint64())
	require.ErrorIs(t, err, ethereum.NotFound)

	l2ClaimedBlockNumber := l2SafeHead.Number.Uint64()
	syncedRollupClient := env.Sequencer.RollupClient()
	l2PreBlockNum := l2ClaimedBlockNumber - 1
	preRoot, err := syncedRollupClient.OutputAtBlock(t.Ctx(), l2PreBlockNum)
	require.NoError(t, err)
	claimRoot, err := syncedRollupClient.OutputAtBlock(t.Ctx(), l2ClaimedBlockNumber)
	require.NoError(t, err)
	l2Claim := common.Hash(claimRoot.OutputRoot)
	l2Head := l2SafeHead.ParentHash
	l2AgreedOutputRoot := common.Hash(preRoot.OutputRoot)
	chainID := eth.ChainIDFromBig(verifier.RollupCfg.L2ChainID)

	fixtureInputs := &helpers.FixtureInputs{
		L2BlockNumber:  l2ClaimedBlockNumber,
		L2Claim:        l2Claim,
		L2Head:         l2Head,
		L2OutputRoot:   l2AgreedOutputRoot,
		L2ChainID:      chainID,
		L1Head:         l1Head.Hash(),
		AgreedPrestate: nil, // not used for block execution
		InteropEnabled: false,
		L2Sources: []*helpers.FaultProofProgramL2Source{{
			Node:        verifier,
			Engine:      verifierEngine,
			ChainConfig: verifierEngine.L2Chain().Config(),
		}},
	}
	programCfg := helpers.NewOpProgramCfg(fixtureInputs)
	kv := kvstore.NewMemKV()
	prefetcher, err := helpers.CreateInprocessPrefetcher(
		t,
		t.Ctx(),
		testlog.Logger(t, log.LevelDebug).New("role", "prefetcher"),
		env.Miner,
		kv,
		programCfg,
		fixtureInputs,
	)
	require.NoError(t, err)

	oracle := func(key preimage.Key) []byte {
		value, err := prefetcher.GetPreimage(t.Ctx(), key.PreimageKey())
		require.NoError(t, err)
		return value
	}
	hinter := func(hint preimage.Hint) {
		err := prefetcher.Hint(hint.Hint())
		require.NoError(t, err)
	}
	l2Oracle := l2.NewPreimageOracle(preimage.OracleFn(oracle), preimage.HinterFn(hinter), false)

	block := l2Oracle.BlockDataByHash(l2SafeHead.ParentHash, l2SafeHead.Hash(), chainID)
	require.Equal(t, l2SafeHead.Hash(), block.Hash())

	// It's enough to assert that these functions do not panic
	txs := l2Oracle.LoadTransactions(l2SafeHead.Hash(), l2SafeHead.TxHash, chainID)
	require.NotNil(t, txs)
	_, receipts := l2Oracle.ReceiptsByBlockHash(l2SafeHead.Hash(), chainID)
	require.NotNil(t, receipts)
}

func createVerifier(t actionsHelpers.Testing, env *helpers.L2FaultProofEnv) (*actionsHelpers.L2Verifier, *actionsHelpers.L2Engine) {
	logger := testlog.Logger(t, log.LevelInfo)
	l1 := env.Miner.L1ClientSimple(t)
	blobSrc := env.Miner.BlobStore()
	jwtPath := e2eutils.WriteDefaultJWT(t)
	engine := actionsHelpers.NewL2Engine(t, logger.New("role", "verifier-2"), env.Sd.L2Cfg, jwtPath)
	l2EngineCl, err := sources.NewEngineClient(engine.RPCClient(), logger, nil, sources.EngineClientDefaultConfig(env.Sd.RollupCfg))
	require.NoError(t, err)
	return actionsHelpers.NewL2Verifier(
		t,
		logger.New("role", "verifier-2"),
		l1,
		blobSrc,
		altda.Disabled,
		l2EngineCl,
		env.Sd.RollupCfg,
		&sync.Config{},
		safedb.Disabled,
	), engine
}
