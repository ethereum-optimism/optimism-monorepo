package upgrades

import (
	"testing"

	"github.com/ethereum-optimism/optimism/op-e2e/actions/helpers"
	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
)

func TestPectraForkAfterGenesis(gt *testing.T) {
	t := helpers.NewDefaultTesting(gt)
	dp := e2eutils.MakeDeployParams(t, helpers.DefaultRollupTestParams())
	offset := hexutil.Uint64(24)
	dp.DeployConfig.L1PragueTimeOffset = &offset
	sd := e2eutils.Setup(t, dp, helpers.DefaultAlloc)
	log := testlog.Logger(t, log.LevelDebug)
	_, _, miner, sequencer, _, verifier, _, batcher := helpers.SetupReorgTestActors(t, dp, sd, log)

	// utils
	checkVerifierDerivedToL1Head := func(t helpers.StatefulTesting) {
		l1Head := miner.L1Chain().CurrentBlock()
		currentL1 := verifier.SyncStatus().CurrentL1
		require.Equal(t, l1Head.Number.Int64(), int64(currentL1.Number), "verifier should derive up to and including the L1 head")
		require.Equal(t, l1Head.Hash(), currentL1.Hash, "verifier should derive up to and including the L1 head")
	}

	buildUnsafeL2AndSubmit := func(useSetCode bool) {
		sequencer.ActL2EmptyBlock(t)
		miner.ActL1StartBlock(12)(t)
		if useSetCode {
			batcher.ActBufferAll(t)
			batcher.ActL2ChannelClose(t)
			batcher.ActSubmitSetCodeTx(t)
		} else {
			batcher.ActSubmitAll(t)
		}
		miner.ActL1IncludeTx(batcher.BatcherAddr)(t)
		miner.ActL1EndBlock(t)
	}

	checkPectraStatusOnL1 := func(active bool) {
		l1Head := miner.L1Chain().CurrentBlock()
		if active {
			require.True(t, sd.L1Cfg.Config.IsPrague(l1Head.Number, l1Head.Time), "Prague should be active")
			require.NotNil(t, l1Head.RequestsHash, "Prague header requests hash should be non-nil")
		} else {
			require.False(t, sd.L1Cfg.Config.IsPrague(l1Head.Number, l1Head.Time), "Prague should not be active yet")
			require.Nil(t, l1Head.RequestsHash, "Prague header requests hash should be nil")
		}
	}

	syncVerifierAndCheck := func(t helpers.StatefulTesting) {
		verifier.ActL1HeadSignal(t)
		verifier.ActL2PipelineFull(t)
		checkVerifierDerivedToL1Head(t)
	}

	// Check initially Pectra is not activated
	checkPectraStatusOnL1(false)

	// Start op-nodes
	sequencer.ActL2PipelineFull(t)
	verifier.ActL2PipelineFull(t)

	// Build empty L1 blocks, crossing the fork boundary
	miner.ActEmptyBlock(t)
	miner.ActEmptyBlock(t) // Pectra activates here
	miner.ActEmptyBlock(t)

	// Check that Pectra is active on L1
	checkPectraStatusOnL1(true)

	// Build L2 unsafe chain and batch it to L1 using calldata txs
	buildUnsafeL2AndSubmit(false)

	// Check verifier derived from Prague L1 blocks
	syncVerifierAndCheck(t)

	// Build L2 unsafe chain, get the batcher to submit as usual, but using an EIP-7702 transaction
	// https://github.com/ethereum/EIPs/blob/master/EIPS/eip-7702.md
	buildUnsafeL2AndSubmit(true)

	// Cache safe head before verifier sync
	safeL1Before := verifier.SyncStatus().SafeL2

	// Check verifier did not panic and ignored the EIP 7702 Set Code Transaction
	syncVerifierAndCheck(t)

	// Check safe head did not change
	safeHeadAfter := verifier.SyncStatus().SafeL2
	require.Equal(t, safeL1Before, safeHeadAfter, "safe head should not have changed (set code batcher tx ignored)")
}
