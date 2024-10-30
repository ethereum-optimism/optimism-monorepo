package upgrades

import (
	"testing"

	"github.com/ethereum-optimism/optimism/op-e2e/actions/helpers"
	"github.com/ethereum-optimism/optimism/op-e2e/system/e2esys"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/stretchr/testify/require"
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
