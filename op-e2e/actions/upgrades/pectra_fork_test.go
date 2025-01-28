package upgrades

import (
	"testing"

	"github.com/ethereum-optimism/optimism/op-e2e/actions/helpers"
	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"
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

	l1Head := miner.L1Chain().CurrentBlock()
	require.False(t, sd.L1Cfg.Config.IsPrague(l1Head.Number, l1Head.Time), "Prague should not be active yet")
	require.Nil(t, l1Head.RequestsHash, "Prague header requests hash should be nil")

	// start op-nodes
	sequencer.ActL2PipelineFull(t)
	verifier.ActL2PipelineFull(t)

	// build empty L1 blocks, crossing the fork boundary
	miner.ActL1SetFeeRecipient(common.Address{'A', 0})
	miner.ActEmptyBlock(t)
	miner.ActEmptyBlock(t) // Pectra activates here
	miner.ActEmptyBlock(t)

	// verify Pectra is active on L1
	l1Head = miner.L1Chain().CurrentBlock()
	require.True(t, sd.L1Cfg.Config.IsPrague(l1Head.Number, l1Head.Time), "Prague should be active")
	require.NotNil(t, l1Head.RequestsHash, "Prague header requests hash should be non-nil")

	// build L2 chain up to and including L2 blocks referencing Pectra L1 blocks
	sequencer.ActL1HeadSignal(t)
	sequencer.ActBuildToL1Head(t)
	miner.ActL1StartBlock(12)(t)
	batcher.ActSubmitAll(t)
	miner.ActL1IncludeTx(batcher.BatcherAddr)(t)
	miner.ActL1EndBlock(t)

	// sync verifier
	verifier.ActL1HeadSignal(t)
	verifier.ActL2PipelineFull(t)
	// verify verifier accepted Prague L1 inputs
	require.Equal(t, l1Head.Hash(), verifier.SyncStatus().SafeL2.L1Origin.Hash, "verifier synced L1 chain that includes Prague headers")
	require.Equal(t, sequencer.SyncStatus().UnsafeL2, verifier.SyncStatus().UnsafeL2, "verifier and sequencer agree")

	// TODO stash safe head block hash

	t.Log("submitting EIP 7702 Set Code Batcher Transaction...")
	// submit an EIP 7702 Set Code Transaction
	// https://github.com/ethereum/EIPs/blob/master/EIPS/eip-7702.md
	miner.ActL1StartBlock(12)(t)
	batcher.ActSubmitSetCodeTx(t)
	miner.ActL1IncludeTx(batcher.BatcherAddr)(t)
	miner.ActL1EndBlock(t)

	// sync verifier
	verifier.ActL1HeadSignal(t)
	verifier.ActL2PipelineFull(t)
	// verify verifier did not panic and ignored the EIP 7702 Set Code Transaction
	require.Equal(t, l1Head.Hash(), verifier.SyncStatus().SafeL2.L1Origin.Hash, "verifier synced L1 chain that includes Prague headers")
	require.Equal(t, sequencer.SyncStatus().UnsafeL2, verifier.SyncStatus().UnsafeL2, "verifier and sequencer agree")

	// TODO check sade head has not changed.
}
