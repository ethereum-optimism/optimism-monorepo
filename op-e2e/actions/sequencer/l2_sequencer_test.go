package sequencer

import (
	"math/big"
	"testing"

	"github.com/ethereum-optimism/optimism/op-e2e/config"

	"github.com/ethereum-optimism/optimism/op-e2e/actions/helpers"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
)

func TestL2Sequencer_SequencerDrift(gt *testing.T) {
	t := helpers.NewDefaultTesting(gt)
	p := &e2eutils.TestParams{
		MaxSequencerDrift:   20, // larger than L1 block time we simulate in this test (12)
		SequencerWindowSize: 24,
		ChannelTimeout:      20,
		L1BlockTime:         12,
		AllocType:           config.AllocTypeStandard,
	}
	dp := e2eutils.MakeDeployParams(t, p)
	sd := e2eutils.Setup(t, dp, helpers.DefaultAlloc)
	log := testlog.Logger(t, log.LevelDebug)
	miner, engine, sequencer := helpers.SetupSequencerTest(t, sd, log)
	miner.ActL1SetFeeRecipient(common.Address{'A'})

	sequencer.ActL2PipelineFull(t)

	signer := types.LatestSigner(sd.L2Cfg.Config)
	cl := engine.EthClient()
	aliceTx := func() {
		n, err := cl.PendingNonceAt(t.Ctx(), dp.Addresses.Alice)
		require.NoError(t, err)
		tx := types.MustSignNewTx(dp.Secrets.Alice, signer, &types.DynamicFeeTx{
			ChainID:   sd.L2Cfg.Config.ChainID,
			Nonce:     n,
			GasTipCap: big.NewInt(2 * params.GWei),
			GasFeeCap: new(big.Int).Add(miner.L1Chain().CurrentBlock().BaseFee, big.NewInt(2*params.GWei)),
			Gas:       params.TxGas,
			To:        &dp.Addresses.Bob,
			Value:     e2eutils.Ether(2),
		})
		require.NoError(gt, cl.SendTransaction(t.Ctx(), tx))
	}
	makeL2BlockWithAliceTx := func() {
		aliceTx()
		sequencer.ActL2StartBlock(t)
		engine.ActL2IncludeTx(dp.Addresses.Alice)(t) // include a test tx from alice
		sequencer.ActL2EndBlock(t)
	}

	// L1 makes a block
	miner.ActL1StartBlock(12)(t)
	miner.ActL1EndBlock(t)
	sequencer.ActL1HeadSignal(t)
	origin := miner.L1Chain().CurrentBlock()

	// L2 makes blocks to catch up
	for sequencer.SyncStatus().UnsafeL2.Time+sd.RollupCfg.BlockTime < origin.Time {
		makeL2BlockWithAliceTx()
		require.Equal(t, uint64(0), sequencer.SyncStatus().UnsafeL2.L1Origin.Number, "no L1 origin change before time matches")
	}
	// Check that we adopted the origin as soon as we could (conf depth is 0)
	makeL2BlockWithAliceTx()
	require.Equal(t, uint64(1), sequencer.SyncStatus().UnsafeL2.L1Origin.Number, "L1 origin changes as soon as L2 time equals or exceeds L1 time")

	miner.ActL1StartBlock(12)(t)
	miner.ActL1EndBlock(t)
	sequencer.ActL1HeadSignal(t)

	// Make blocks up till the sequencer drift is about to surpass, but keep the old L1 origin
	for sequencer.SyncStatus().UnsafeL2.Time+sd.RollupCfg.BlockTime <= origin.Time+sd.ChainSpec.MaxSequencerDrift(origin.Time) {
		sequencer.ActL2KeepL1Origin(t)
		makeL2BlockWithAliceTx()
		require.Equal(t, uint64(1), sequencer.SyncStatus().UnsafeL2.L1Origin.Number, "expected to keep old L1 origin")
	}

	// We passed the sequencer drift: we can still keep the old origin, but can't include any txs
	sequencer.ActL2KeepL1Origin(t)
	sequencer.ActL2StartBlock(t)
	require.True(t, engine.EngineApi.ForcedEmpty(), "engine should not be allowed to include anything after sequencer drift is surpassed")
}

// TestL2Sequencer_SequencerOnlyReorg regression-tests a Goerli halt where the sequencer
// would build an unsafe L2 block with a L1 origin that then gets reorged out,
// while the verifier-codepath only ever sees the valid post-reorg L1 chain.
func TestL2Sequencer_SequencerOnlyReorg(gt *testing.T) {
	t := helpers.NewDefaultTesting(gt)
	dp := e2eutils.MakeDeployParams(t, helpers.DefaultRollupTestParams())
	sd := e2eutils.Setup(t, dp, helpers.DefaultAlloc)
	log := testlog.Logger(t, log.LevelDebug)
	miner, _, sequencer := helpers.SetupSequencerTest(t, sd, log)

	// Sequencer at first only recognizes the genesis as safe.
	// The rest of the L1 chain will be incorporated as L1 origins into unsafe L2 blocks.
	sequencer.ActL2PipelineFull(t)

	// build L1 block with coinbase A
	miner.ActL1SetFeeRecipient(common.Address{'A'})
	miner.ActEmptyBlock(t)

	// sequencer builds L2 blocks, until (incl.) it creates a L2 block with a L1 origin that has A as coinbase address
	sequencer.ActL1HeadSignal(t)
	sequencer.ActBuildToL1HeadUnsafe(t)

	// If sequencer does not pick up on pre-reorg chain in derivation,
	// then derivation won't see the difference in L1 chains,
	// and not trigger a reorg if we traverse from 0 to the new chain later on
	// (but would once it gets to consolidate unsafe head later).
	sequencer.ActL2PipelineFull(t)
	status := sequencer.SyncStatus()
	require.Zero(t, status.SafeL2.L1Origin.Number, "no safe head progress")
	require.Equal(t, status.HeadL1.Hash, status.UnsafeL2.L1Origin.Hash, "have head L1 origin")
	require.NotZero(t, status.UnsafeL2.L1Origin.Number, "have head L1 origin")
	// reorg out block with coinbase A, and make a block with coinbase B
	miner.ActL1RewindToParent(t)
	miner.ActL1SetFeeRecipient(common.Address{'B'})
	miner.ActEmptyBlock(t)

	// and a second block, for derivation to pick up on the new L1 chain
	// (height is used as heuristic to not flip-flop between chains too frequently)
	miner.ActEmptyBlock(t)

	// Make the sequencer aware of the new head, and try to sync it.
	// Since the safe chain never incorporated the now reorged L1 block with coinbase A,
	// it will sync the new L1 chain fine.
	// No batches are submitted yet however,
	// so it'll keep the L2 block with the old L1 origin, since no conflict is detected.
	sequencer.ActL1HeadSignal(t)

	postReorgStatus := sequencer.SyncStatus()
	require.Zero(t, postReorgStatus.SafeL2.L1Origin.Number, "no safe head progress")
	require.NotEqual(t, postReorgStatus.HeadL1.Hash, postReorgStatus.UnsafeL2.L1Origin.Hash, "no longer have head L1 origin")

	sequencer.ActL2PipelineFull(t)
	// Verifier should detect the inconsistency of the L1 origin and reset the pipeline to follow the reorg
	newStatus := sequencer.SyncStatus()
	require.Zero(t, newStatus.UnsafeL2.L1Origin.Number, "back to genesis block with good L1 origin, drop old unsafe L2 chain with bad L1 origins")
	require.NotEqual(t, status.HeadL1.Hash, newStatus.HeadL1.Hash, "did see the new L1 head change")
	require.Equal(t, newStatus.HeadL1.Hash, newStatus.CurrentL1.Hash, "did sync the new L1 head as verifier")

	// the block N+1 cannot build on the old N which still refers to the now orphaned L1 origin
	require.Equal(t, status.UnsafeL2.L1Origin.Number, newStatus.HeadL1.Number-1, "seeing N+1 to attempt to build on N")
	require.NotEqual(t, status.UnsafeL2.L1Origin.Hash, newStatus.HeadL1.ParentHash, "but N+1 cannot fit on N")

	// Can build new L2 blocks with good L1 origin
	sequencer.ActBuildToL1HeadUnsafe(t)
	require.Equal(t, newStatus.HeadL1.Hash, sequencer.SyncStatus().UnsafeL2.L1Origin.Hash, "build L2 chain with new correct L1 origins")
}
