package derivation

import (
	"testing"

	"github.com/ethereum-optimism/optimism/op-e2e/config"

	altda "github.com/ethereum-optimism/optimism/op-alt-da"
	batcherFlags "github.com/ethereum-optimism/optimism/op-batcher/flags"
	"github.com/ethereum-optimism/optimism/op-e2e/actions/helpers"
	upgradesHelpers "github.com/ethereum-optimism/optimism/op-e2e/actions/upgrades/helpers"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils"
	"github.com/ethereum-optimism/optimism/op-node/node/safedb"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum-optimism/optimism/op-node/rollup/sync"
	"github.com/ethereum-optimism/optimism/op-service/sources"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"
)

// TestDeriveChainFromNearL1Genesis tests a corner case where when the derivation pipeline starts, the
// safe head has an L1 origin of block 1. The derivation then starts with pipeline origin of L1 genesis,
// just one block prior to the origin of the safe head.
// This is a regression test, previously the pipeline encountered got stuck in a reset loop with the error:
// buffered L1 chain epoch %s in batch queue does not match safe head origin %s
func TestDeriveChainFromNearL1Genesis(gt *testing.T) {
	t := helpers.NewDefaultTesting(gt)
	p := &e2eutils.TestParams{
		MaxSequencerDrift:   20, // larger than L1 block time we simulate in this test (12)
		SequencerWindowSize: 24,
		ChannelTimeout:      20,
		L1BlockTime:         12,
		AllocType:           config.AllocTypeStandard,
	}
	dp := e2eutils.MakeDeployParams(t, p)
	// do not activate Delta hardfork for verifier
	upgradesHelpers.ApplyDeltaTimeOffset(dp, nil)
	sd := e2eutils.Setup(t, dp, helpers.DefaultAlloc)
	logger := testlog.Logger(t, log.LevelInfo)
	miner, seqEngine, sequencer := helpers.SetupSequencerTest(t, sd, logger)

	miner.ActEmptyBlock(t)
	require.EqualValues(gt, 1, miner.L1Chain().CurrentBlock().Number.Uint64())

	ref, err := derive.L2BlockToBlockRef(sequencer.RollupCfg, seqEngine.L2Chain().Genesis())
	require.NoError(gt, err)
	require.EqualValues(gt, 0, ref.L1Origin.Number)

	sequencer.ActL1HeadSignal(t)
	sequencer.ActBuildToL1Head(t)
	l2BlockNum := seqEngine.L2Chain().CurrentBlock().Number.Uint64()
	ref, err = derive.L2BlockToBlockRef(sequencer.RollupCfg, seqEngine.L2Chain().GetBlockByNumber(l2BlockNum))
	require.NoError(gt, err)
	require.EqualValues(gt, 1, ref.L1Origin.Number)

	miner.ActEmptyBlock(t)

	rollupSeqCl := sequencer.RollupClient()
	// Force batcher to submit SingularBatches to L1.
	batcher := helpers.NewL2Batcher(logger, sd.RollupCfg, &helpers.BatcherCfg{
		MinL1TxSize:          0,
		MaxL1TxSize:          128_000,
		BatcherKey:           dp.Secrets.Batcher,
		DataAvailabilityType: batcherFlags.CalldataType,
	}, rollupSeqCl, miner.EthClient(), seqEngine.EthClient(), seqEngine.EngineClient(t, sd.RollupCfg))

	batcher.ActSubmitAll(t)
	require.EqualValues(gt, l2BlockNum, batcher.L2BufferedBlock.Number)

	// confirm batch on L1
	miner.ActL1StartBlock(12)(t)
	miner.ActL1IncludeTx(dp.Addresses.Batcher)(t)
	miner.ActL1EndBlock(t)
	bl := miner.L1Chain().CurrentBlock()
	logger.Info("Produced L1 block with batch",
		"num", miner.L1Chain().CurrentBlock().Number.Uint64(),
		"txs", len(miner.L1Chain().GetBlockByHash(bl.Hash()).Transactions()))

	// Process batches so safe head updates
	sequencer.ActL1HeadSignal(t)
	sequencer.ActL2PipelineFull(t)
	require.EqualValues(gt, l2BlockNum, seqEngine.L2Chain().CurrentSafeBlock().Number.Uint64())

	// Finalize L1 and process so L2 finalized updates
	miner.ActL1Safe(t, miner.L1Chain().CurrentBlock().Number.Uint64())
	miner.ActL1Finalize(t, miner.L1Chain().CurrentBlock().Number.Uint64())
	sequencer.ActL1SafeSignal(t)
	sequencer.ActL1FinalizedSignal(t)
	sequencer.ActL2PipelineFull(t)
	require.EqualValues(gt, l2BlockNum, seqEngine.L2Chain().CurrentFinalBlock().Number.Uint64())

	// Create a new verifier using the existing engine so it already has the safe and finalized heads set.
	// This is the same situation as if op-node restarted at this point.
	l2Cl, err := sources.NewEngineClient(seqEngine.RPCClient(), logger, nil, sources.EngineClientDefaultConfig(sd.RollupCfg))
	require.NoError(gt, err)
	verifier := helpers.NewL2Verifier(t, logger, miner.L1Client(t, sd.RollupCfg), miner.BlobStore(), altda.Disabled,
		l2Cl, sequencer.RollupCfg, &sync.Config{}, safedb.Disabled, nil)
	verifier.ActL2PipelineFull(t) // Should not get stuck in a reset loop forever
	require.EqualValues(gt, l2BlockNum, seqEngine.L2Chain().CurrentSafeBlock().Number.Uint64())
	require.EqualValues(gt, l2BlockNum, seqEngine.L2Chain().CurrentFinalBlock().Number.Uint64())
	syncStatus := verifier.SyncStatus()
	require.EqualValues(gt, l2BlockNum, syncStatus.SafeL2.Number)
	require.EqualValues(gt, l2BlockNum, syncStatus.FinalizedL2.Number)
}
