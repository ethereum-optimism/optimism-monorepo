package upgrades

import (
	"context"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/core/types"

	"github.com/ethereum-optimism/optimism/op-e2e/actions/helpers"
	"github.com/ethereum-optimism/optimism/op-e2e/system/e2esys"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
)

func TestHoloceneActivationAtGenesis(gt *testing.T) {
	t := helpers.NewDefaultTesting(gt)
	env := helpers.SetupEnv(t, helpers.WithActiveGenesisFork(rollup.Holocene))

	// Start op-nodes
	env.Seq.ActL2PipelineFull(t)
	env.Verifier.ActL2PipelineFull(t)

	// Verify Holocene is active at genesis
	l2Head := env.Seq.L2Unsafe()
	require.NotZero(t, l2Head.Hash)
	require.True(t, env.SetupData.RollupCfg.IsHolocene(l2Head.Time), "Holocene should be active at genesis")

	// build empty L1 block
	env.Miner.ActEmptyBlock(t)

	// Build L2 chain and advance safe head
	env.Seq.ActL1HeadSignal(t)
	env.Seq.ActBuildToL1Head(t)

	// verify in logs that correct stage got activated
	recs := env.Logs.FindLogs(testlog.NewMessageContainsFilter("activating Holocene stage during reset"), testlog.NewAttributesFilter("role", e2esys.RoleSeq))
	require.Len(t, recs, 2)
	recs = env.Logs.FindLogs(testlog.NewMessageContainsFilter("activating Holocene stage during reset"), testlog.NewAttributesFilter("role", e2esys.RoleVerif))
	require.Len(t, recs, 2)

	env.ActBatchSubmitAllAndMine(t)

	// verifier picks up the L2 chain that was submitted
	env.Verifier.ActL1HeadSignal(t)
	env.Verifier.ActL2PipelineFull(t)
	require.Equal(t, env.Verifier.L2Safe(), env.Seq.L2Unsafe(), "verifier syncs from sequencer via L1")
	require.NotEqual(t, env.Seq.L2Safe(), env.Seq.L2Unsafe(), "sequencer has not processed L1 yet")
}

func TestHoloceneLateActivationAndReset(gt *testing.T) {
	t := helpers.NewDefaultTesting(gt)
	holoceneOffset := uint64(24)
	env := helpers.SetupEnv(t, helpers.WithActiveFork(rollup.Holocene, holoceneOffset))

	requireHoloceneTransformationLogs := func(role string, expNumLogs int) {
		recs := env.Logs.FindLogs(testlog.NewMessageContainsFilter("transforming to Holocene"), testlog.NewAttributesFilter("role", role))
		require.Len(t, recs, expNumLogs)
		if expNumLogs > 0 {
			fqRecs := env.Logs.FindLogs(testlog.NewMessageFilter("FrameQueue: resetting with Holocene activation"), testlog.NewAttributesFilter("role", role))
			require.Len(t, fqRecs, 1)
		}
	}

	requirePreHoloceneActivationLogs := func(role string, expNumLogs int) {
		recs := env.Logs.FindLogs(testlog.NewMessageContainsFilter("activating pre-Holocene stage during reset"), testlog.NewAttributesFilter("role", role))
		require.Len(t, recs, expNumLogs)
	}

	// Start op-nodes
	env.Seq.ActL2PipelineFull(t)
	env.Verifier.ActL2PipelineFull(t)

	// Verify Holocene is not active at genesis yet
	l2Head := env.Seq.L2Unsafe()
	require.NotZero(t, l2Head.Hash)
	require.True(t, env.SetupData.RollupCfg.IsGranite(l2Head.Time), "Granite should be active at genesis")
	require.False(t, env.SetupData.RollupCfg.IsHolocene(l2Head.Time), "Holocene should not be active at genesis")

	requirePreHoloceneActivationLogs(e2esys.RoleSeq, 2)
	requirePreHoloceneActivationLogs(e2esys.RoleVerif, 2)
	// Verify no stage transformations took place yet
	requireHoloceneTransformationLogs(e2esys.RoleSeq, 0)
	requireHoloceneTransformationLogs(e2esys.RoleVerif, 0)

	env.Seq.ActL2EmptyBlock(t)
	l1PreHolocene := env.ActBatchSubmitAllAndMine(t)
	require.False(t, env.SetupData.RollupCfg.IsHolocene(l1PreHolocene.Time()),
		"Holocene should not be active at the first L1 inclusion block")

	// Build a few L2 blocks. We only need the L1 inclusion to advance past Holocene and Holocene
	// shouldn't activate with L2 time.
	env.Seq.ActBuildL2ToHolocene(t)

	// verify in logs that stage transformations hasn't happened yet, activates by L1 inclusion block
	requireHoloceneTransformationLogs(e2esys.RoleSeq, 0)
	requireHoloceneTransformationLogs(e2esys.RoleVerif, 0)

	// Submit L2
	l1Head := env.ActBatchSubmitAllAndMine(t)
	require.True(t, env.SetupData.RollupCfg.IsHolocene(l1Head.Time()))

	// verifier picks up the L2 chain that was submitted
	env.Verifier.ActL1HeadSignal(t)
	env.Verifier.ActL2PipelineFull(t)
	l2Safe := env.Verifier.L2Safe()
	require.Equal(t, l2Safe, env.Seq.L2Unsafe(), "verifier syncs from sequencer via L1")
	require.NotEqual(t, env.Seq.L2Safe(), env.Seq.L2Unsafe(), "sequencer has not processed L1 yet")
	require.True(t, env.SetupData.RollupCfg.IsHolocene(l2Safe.Time), "Holocene should now be active")
	requireHoloceneTransformationLogs(e2esys.RoleSeq, 0)
	requireHoloceneTransformationLogs(e2esys.RoleVerif, 2)

	// sequencer also picks up L2 safe chain
	env.Seq.ActL1HeadSignal(t)
	env.Seq.ActL2PipelineFull(t)
	requireHoloceneTransformationLogs(e2esys.RoleSeq, 2)
	require.Equal(t, env.Seq.L2Safe(), env.Seq.L2Unsafe(), "sequencer has processed L1")

	// reorg L1 without batch submission
	env.Miner.ActL1RewindToParent(t)
	env.Miner.ActEmptyBlock(t)
	env.Miner.ActEmptyBlock(t)

	env.Seq.ActL1HeadSignal(t)
	env.Verifier.ActL1HeadSignal(t)
	env.Seq.ActL2PipelineFull(t)
	env.Verifier.ActL2PipelineFull(t)

	// duplicate activation logs
	requirePreHoloceneActivationLogs(e2esys.RoleSeq, 4)
	requirePreHoloceneActivationLogs(e2esys.RoleVerif, 4)
}

func TestHoloceneInvalidPayload(gt *testing.T) {
	t := helpers.NewDefaultTesting(gt)
	env := helpers.SetupEnv(t, helpers.WithActiveGenesisFork(rollup.Holocene))
	ctx := context.Background()

	requireDepositOnlyLogs := func(role string, expNumLogs int) {
		t.Helper()
		recs := env.Logs.FindLogs(testlog.NewMessageContainsFilter("deposits-only attributes"), testlog.NewAttributesFilter("role", role))
		require.Len(t, recs, expNumLogs)
	}

	// Start op-nodes
	env.Seq.ActL2PipelineFull(t)

	// generate and batch buffer two empty blocks
	env.Seq.ActL2EmptyBlock(t) // 1 - genesis is 0
	env.Batcher.ActL2BatchBuffer(t)
	env.Seq.ActL2EmptyBlock(t) // 2
	env.Batcher.ActL2BatchBuffer(t)

	// send and include a single transaction
	env.Alice.L2.ActResetTxOpts(t)
	env.Alice.L2.ActSetTxToAddr(&env.DeployParams.Addresses.Bob)
	env.Alice.L2.ActMakeTx(t)

	env.Seq.ActL2StartBlock(t)
	env.SeqEngine.ActL2IncludeTx(env.Alice.Address())(t)
	env.Seq.ActL2EndBlock(t) // 3
	env.Alice.L2.ActCheckReceiptStatusOfLastTx(true)(t)
	l2Unsafe := env.Seq.L2Unsafe()
	const invalidNum = 3
	require.EqualValues(t, invalidNum, l2Unsafe.Number)
	b, err := env.SeqEngine.EthClient().BlockByNumber(ctx, big.NewInt(invalidNum))
	require.NoError(t, err)
	require.Len(t, b.Transactions(), 2)

	// buffer into the batcher, invalidating the tx via signature zeroing
	env.Batcher.ActL2BatchBuffer(t, func(block *types.Block) *types.Block {
		// Replace the tx with one that has a bad signature.
		txs := block.Transactions()
		newTx, err := txs[1].WithSignature(env.Alice.L2.Signer(), make([]byte, 65))
		require.NoError(t, err)
		txs[1] = newTx
		return block
	})

	// generate two more empty blocks
	env.Seq.ActL2EmptyBlock(t) // 4
	env.Seq.ActL2EmptyBlock(t) // 5
	require.EqualValues(t, 5, env.Seq.L2Unsafe().Number)

	// submit it all
	env.ActBatchSubmitAllAndMine(t)

	// derive chain on sequencer
	env.Seq.ActL1HeadSignal(t)
	env.Seq.ActL2PipelineFull(t)

	l2Safe := env.Seq.L2Safe()
	require.EqualValues(t, invalidNum, l2Safe.Number)
	require.NotEqual(t, l2Safe.Hash, l2Unsafe.Hash, // old L2Unsafe above
		"block-3 should have been replaced by deposit-only version")
	requireDepositOnlyLogs(e2esys.RoleSeq, 2)
	require.Equal(t, l2Safe, env.Seq.L2Unsafe(), "unsafe chain should have reorg'd")
	b, err = env.SeqEngine.EthClient().BlockByNumber(ctx, big.NewInt(invalidNum))
	require.NoError(t, err)
	require.Len(t, b.Transactions(), 1)

	// test that building on top of reorg'd chain and deriving further works

	env.Seq.ActL2EmptyBlock(t) // 4
	env.Seq.ActL2EmptyBlock(t) // 5
	l2Unsafe = env.Seq.L2Unsafe()
	require.EqualValues(t, 5, l2Unsafe.Number)

	env.Batcher.Reset() // need to reset batcher to become aware of reorg
	env.ActBatchSubmitAllAndMine(t)
	env.Seq.ActL1HeadSignal(t)
	env.Seq.ActL2PipelineFull(t)
	require.Equal(t, l2Unsafe, env.Seq.L2Safe())
}
