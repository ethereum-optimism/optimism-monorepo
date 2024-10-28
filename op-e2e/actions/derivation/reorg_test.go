package derivation

import (
	"math/big"
	"math/rand"
	"path"
	"testing"

	"github.com/ethereum-optimism/optimism/op-e2e/config"

	actionsHelpers "github.com/ethereum-optimism/optimism/op-e2e/actions/helpers"
	upgradesHelpers "github.com/ethereum-optimism/optimism/op-e2e/actions/upgrades/helpers"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/stretchr/testify/require"

	altda "github.com/ethereum-optimism/optimism/op-alt-da"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils"
	"github.com/ethereum-optimism/optimism/op-service/client"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/sources"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
)

// TestReorgBatchType run each reorg-related test case in singular batch mode and span batch mode.
func TestReorgBatchType(t *testing.T) {
	tests := []struct {
		name string
		f    func(gt *testing.T, deltaTimeOffset *hexutil.Uint64)
	}{
		{"ReorgOrphanBlock", ReorgOrphanBlock},
		{"ReorgFlipFlop", ReorgFlipFlop},
		{"DeepReorg", DeepReorg},
		{"RestartOpGeth", RestartOpGeth},
		{"ConflictingL2Blocks", ConflictingL2Blocks},
		{"SyncAfterReorg", SyncAfterReorg},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name+"_SingularBatch", func(t *testing.T) {
			test.f(t, nil)
		})
	}

	deltaTimeOffset := hexutil.Uint64(0)
	for _, test := range tests {
		test := test
		t.Run(test.name+"_SpanBatch", func(t *testing.T) {
			test.f(t, &deltaTimeOffset)
		})
	}
}

func ReorgOrphanBlock(gt *testing.T, deltaTimeOffset *hexutil.Uint64) {
	t := actionsHelpers.NewDefaultTesting(gt)
	sd, _, miner, sequencer, _, verifier, verifierEng, batcher := actionsHelpers.SetupReorgTest(t, actionsHelpers.DefaultRollupTestParams(), deltaTimeOffset)
	verifEngClient := verifierEng.EngineClient(t, sd.RollupCfg)

	sequencer.ActL2PipelineFull(t)
	verifier.ActL2PipelineFull(t)

	// build empty L1 block
	miner.ActEmptyBlock(t)

	// Create L2 blocks, and reference the L1 head as origin
	sequencer.ActL1HeadSignal(t)
	sequencer.ActBuildToL1Head(t)

	// submit all new L2 blocks
	batcher.ActSubmitAll(t)

	// new L1 block with L2 batch
	miner.ActL1StartBlock(12)(t)
	miner.ActL1IncludeTx(sd.RollupCfg.Genesis.SystemConfig.BatcherAddr)(t)
	batchTx := miner.L1Transactions[0]
	miner.ActL1EndBlock(t)

	// verifier picks up the L2 chain that was submitted
	verifier.ActL1HeadSignal(t)
	verifier.ActL2PipelineFull(t)
	require.Equal(t, verifier.L2Safe(), sequencer.L2Unsafe(), "verifier syncs from sequencer via L1")
	require.NotEqual(t, sequencer.L2Safe(), sequencer.L2Unsafe(), "sequencer has not processed L1 yet")

	// orphan the L1 block that included the batch tx, and build a new different L1 block
	miner.ActL1RewindToParent(t)
	miner.ActL1SetFeeRecipient(common.Address{'B'})
	miner.ActEmptyBlock(t)
	miner.ActEmptyBlock(t) // needs to be a longer chain for reorg to be applied. TODO: maybe more aggressively react to reorgs to shorter chains?

	// sync verifier again. The L1 reorg excluded the batch, so now the previous L2 chain should be unsafe again.
	// However, the L2 chain can still be canonical later, since it did not reference the reorged L1 block
	verifier.ActL1HeadSignal(t)
	verifier.ActL2PipelineFull(t)
	require.Equal(t, verifier.L2Safe(), sequencer.L2Safe(), "verifier rewinds safe when L1 reorgs out batch")
	ref, err := verifEngClient.L2BlockRefByLabel(t.Ctx(), eth.Safe)
	require.NoError(t, err)
	require.Equal(t, verifier.L2Safe(), ref, "verifier engine matches rollup client")

	// Now replay the batch tx in a new L1 block
	miner.ActL1StartBlock(12)(t)
	miner.ActL1SetFeeRecipient(common.Address{'C'})
	// note: the geth tx pool reorgLoop is too slow (responds to chain head events, but async),
	// and there's no way to manually trigger runReorg, so we re-insert it ourselves.
	require.NoError(t, miner.Eth.TxPool().Add([]*types.Transaction{batchTx}, true, true)[0])
	// need to re-insert previously included tx into the block
	miner.ActL1IncludeTx(sd.RollupCfg.Genesis.SystemConfig.BatcherAddr)(t)
	miner.ActL1EndBlock(t)

	// sync the verifier again: now it should be safe again
	verifier.ActL1HeadSignal(t)
	verifier.ActL2PipelineFull(t)
	require.Equal(t, verifier.L2Safe(), sequencer.L2Unsafe(), "verifier syncs from sequencer via replayed batch on L1")
	ref, err = verifEngClient.L2BlockRefByLabel(t.Ctx(), eth.Safe)
	require.NoError(t, err)
	require.Equal(t, verifier.L2Safe(), ref, "verifier engine matches rollup client")

	sequencer.ActL1HeadSignal(t)
	sequencer.ActL2PipelineFull(t)
	require.Equal(t, verifier.L2Safe(), sequencer.L2Safe(), "verifier and sequencer see same safe L2 block, while only verifier dealt with the orphan and replay")
}

func ReorgFlipFlop(gt *testing.T, deltaTimeOffset *hexutil.Uint64) {
	t := actionsHelpers.NewDefaultTesting(gt)
	sd, _, miner, sequencer, _, verifier, verifierEng, batcher := actionsHelpers.SetupReorgTest(t, actionsHelpers.DefaultRollupTestParams(), deltaTimeOffset)
	minerCl := miner.L1Client(t, sd.RollupCfg)
	verifEngClient := verifierEng.EngineClient(t, sd.RollupCfg)
	checkVerifEngine := func() {
		ref, err := verifEngClient.L2BlockRefByLabel(t.Ctx(), eth.Safe)
		require.NoError(t, err)
		require.Equal(t, verifier.L2Safe(), ref, "verifier safe head of engine matches rollup client")
	}

	sequencer.ActL2PipelineFull(t)
	verifier.ActL2PipelineFull(t)

	// Start building chain A
	miner.ActL1SetFeeRecipient(common.Address{'A', 0})
	miner.ActEmptyBlock(t)
	blockA0, err := minerCl.L1BlockRefByLabel(t.Ctx(), eth.Unsafe)
	require.NoError(t, err)

	// Create L2 blocks, and reference the L1 head A0 as origin
	sequencer.ActL1HeadSignal(t)
	sequencer.ActBuildToL1Head(t)

	// submit all new L2 blocks
	batcher.ActSubmitAll(t)

	// new L1 block A1 with L2 batch
	miner.ActL1SetFeeRecipient(common.Address{'A', 1})
	miner.ActL1StartBlock(12)(t)
	miner.ActL1IncludeTx(sd.RollupCfg.Genesis.SystemConfig.BatcherAddr)(t)
	batchTxA := miner.L1Transactions[0]
	miner.ActL1EndBlock(t)

	// verifier picks up the L2 chain that was submitted
	verifier.ActL1HeadSignal(t)
	verifier.ActL2PipelineFull(t)
	require.Equal(t, verifier.L2Safe().L1Origin, blockA0.ID(), "verifier syncs L2 chain with L1 A0 origin")
	checkVerifEngine()

	// Flip to chain B!
	miner.ActL1RewindToParent(t) // undo A1
	miner.ActL1RewindToParent(t) // undo A0
	// build B0
	miner.ActL1SetFeeRecipient(common.Address{'B', 0})
	miner.ActEmptyBlock(t)
	blockB0, err := minerCl.L1BlockRefByLabel(t.Ctx(), eth.Unsafe)
	require.NoError(t, err)
	require.Equal(t, blockA0.Number, blockB0.Number, "same height")
	require.NotEqual(t, blockA0.Hash, blockB0.Hash, "different content")

	// re-include the batch tx that submitted L2 chain data that pointed to A0, in the new block B1
	miner.ActL1SetFeeRecipient(common.Address{'B', 1})
	miner.ActL1StartBlock(12)(t)
	require.NoError(t, miner.Eth.TxPool().Add([]*types.Transaction{batchTxA}, true, true)[0])
	miner.ActL1IncludeTx(sd.RollupCfg.Genesis.SystemConfig.BatcherAddr)(t)
	miner.ActL1EndBlock(t)

	// make B2, the reorg is picked up when we have a new longer chain
	miner.ActL1SetFeeRecipient(common.Address{'B', 2})
	miner.ActEmptyBlock(t)
	blockB2, err := minerCl.L1BlockRefByLabel(t.Ctx(), eth.Unsafe)
	require.NoError(t, err)

	// now sync the verifier: some of the batches should be ignored:
	//	The safe head should have a genesis L1 origin, but past genesis, as some L2 blocks were built to get to A0 time
	verifier.ActL1HeadSignal(t)
	verifier.ActL2PipelineFull(t)
	require.Equal(t, sd.RollupCfg.Genesis.L1, verifier.L2Safe().L1Origin, "expected to be back at genesis origin after losing A0 and A1")

	if sd.RollupCfg.DeltaTime == nil {
		// before delta hard fork
		require.NotZero(t, verifier.L2Safe().Number, "still preserving old L2 blocks that did not reference reorged L1 chain (assuming more than one L2 block per L1 block)")
		require.Equal(t, verifier.L2Safe(), verifier.L2Unsafe(), "head is at safe block after L1 reorg")
	} else {
		// after delta hard fork
		require.Zero(t, verifier.L2Safe().Number, "safe head is at genesis block because span batch referenced reorged L1 chain is not accepted")
		require.Equal(t, verifier.L2Unsafe().ID(), sequencer.L2Unsafe().ParentID(), "head is at the highest unsafe block that references canonical L1 chain(genesis block)")
		batcher.L2BufferedBlock = eth.L2BlockRef{} // must reset batcher to resubmit blocks included in the last batch
	}
	checkVerifEngine()

	// and sync the sequencer, then build some new L2 blocks, up to and including with L1 origin B2
	sequencer.ActL1HeadSignal(t)
	sequencer.ActL2PipelineFull(t)
	sequencer.ActBuildToL1Head(t)
	require.Equal(t, sequencer.L2Unsafe().L1Origin, blockB2.ID(), "B2 is the unsafe L1 origin of sequencer now")

	// submit all new L2 blocks for chain B, and include in new block B3
	batcher.ActSubmitAll(t)
	miner.ActL1SetFeeRecipient(common.Address{'B', 3})
	miner.ActL1StartBlock(12)(t)
	miner.ActL1IncludeTx(sd.RollupCfg.Genesis.SystemConfig.BatcherAddr)(t)
	miner.ActL1EndBlock(t)

	// sync the verifier to the L2 chain with origin B2
	verifier.ActL1HeadSignal(t)
	verifier.ActL2PipelineFull(t)
	require.Equal(t, verifier.L2Safe().L1Origin, blockB2.ID(), "B2 is the L1 origin of verifier now")
	require.Equal(t, verifier.L2Unsafe(), sequencer.L2Unsafe(), "verifier unsafe head is reorged along sequencer")
	checkVerifEngine()

	// Flop back to chain A!
	miner.ActL1RewindDepth(4)(t) // B3, B2, B1, B0
	pivotBlock, err := minerCl.L1BlockRefByLabel(t.Ctx(), eth.Unsafe)
	require.NoError(t, err)
	require.Equal(t, sd.RollupCfg.Genesis.L1, pivotBlock.ID(), "back at L1 genesis")
	miner.ActL1SetFeeRecipient(common.Address{'A', 0})
	miner.ActEmptyBlock(t)
	blockA0Again, err := minerCl.L1BlockRefByLabel(t.Ctx(), eth.Unsafe)
	require.NoError(t, err)
	require.Equal(t, blockA0, blockA0Again, "block A0 is back again after flip-flop")

	// Continue to build on A, and replay the old A-chain batch transaction (we can't replay B, since the nonce values would conflict)
	miner.ActL1SetFeeRecipient(common.Address{'A', 1})
	miner.ActEmptyBlock(t)

	miner.ActL1SetFeeRecipient(common.Address{'A', 2})
	miner.ActL1StartBlock(12)(t)
	require.NoError(t, miner.Eth.TxPool().Add([]*types.Transaction{batchTxA}, true, true)[0]) // replay chain A batches, but now in A2 instead of A1
	miner.ActL1IncludeTx(sd.RollupCfg.Genesis.SystemConfig.BatcherAddr)(t)
	miner.ActL1EndBlock(t)

	// build more L1 blocks, so the chain is long enough for reorg to be picked up
	miner.ActL1SetFeeRecipient(common.Address{'A', 3})
	miner.ActEmptyBlock(t)
	miner.ActL1SetFeeRecipient(common.Address{'A', 4})
	miner.ActEmptyBlock(t)
	blockA4, err := minerCl.L1BlockRefByLabel(t.Ctx(), eth.Unsafe)
	require.NoError(t, err)

	// sync verifier, and see if A0 is safe, using the old replayed batch for chain A
	verifier.ActL1HeadSignal(t)
	verifier.ActL2PipelineFull(t)
	require.Equal(t, verifier.L2Safe().L1Origin, blockA0.ID(), "B2 is the L1 origin of verifier now")
	checkVerifEngine()

	// sync sequencer to the replayed L1 chain A
	sequencer.ActL1HeadSignal(t)
	sequencer.ActL2PipelineFull(t)
	require.Equal(t, verifier.L2Safe(), sequencer.L2Safe(), "sequencer reorgs to match verifier again")

	// and adopt the rest of L1 chain A into L2
	sequencer.ActBuildToL1Head(t)

	// submit the new unsafe A blocks
	batcher.ActSubmitAll(t)
	miner.ActL1SetFeeRecipient(common.Address{'A', 5})
	miner.ActL1StartBlock(12)(t)
	miner.ActL1IncludeTx(sd.RollupCfg.Genesis.SystemConfig.BatcherAddr)(t)
	miner.ActL1EndBlock(t)

	// sync verifier to what ths sequencer submitted
	verifier.ActL1HeadSignal(t)
	verifier.ActL2PipelineFull(t)
	require.Equal(t, verifier.L2Safe(), sequencer.L2Unsafe(), "verifier syncs from sequencer")
	require.Equal(t, verifier.L2Safe().L1Origin, blockA4.ID(), "L2 chain origin is A4")
	checkVerifEngine()
}

// Deep Reorg Test
//
// Steps:
//  1. Create an L1 actor
//  2. Ask the L1 actor to build three sequence windows of empty blocks
//     2.a alice submits a transaction on l2 with an l1 origin of block #35
//     2.b in block #50, include the batch that contains the l2 block with alice's transaction as well
//     as all other blocks before it.
//  3. Ask the L2 sequencer to build a chain that references these L1 blocks
//  4. Ask the batch submitter to submit remaining unsafe L2 blocks to L1
//  5. Ask the L1 to include this data
//  6. Rewind chain A 21 blocks
//  7. Ask the L1 actor to build one sequence window + 1 empty blocks on chain B
//  8. Ask the L1 actor to build an empty block in place of the batch submission block on chain A
//  9. Ask the L1 actor to create another empty block so that chain B is longer than chain A
//  10. Ask the L2 sequencer to send a head signal and run one iteration of the derivation pipeline.
//  11. Ask the L2 sequencer build a chain that references chain B's blocks
//  12. Sync the verifier and assert that the L2 safe head L1 origin has caught up with chain B
//  13. Ensure that the parent L2 block of the block that contains Alice's transaction still exists
//     after the L2 has re-derived from chain B.
//  14. Ensure that the L2 block that contained Alice's transaction before the reorg no longer exists.
//
// Chain A
// - 61 blocks total
//   - 60 empty blocks
//   - Alice submits her L2 transaction with an L1 Origin of block #35
//   - In block 50, submit the batch containing the L2 block with Alice's transaction.
//   - Block 61 includes batch with blocks [1, 60]
//
// Verifier
// - Prior to second batch submission, safe head origin is block A50
// - After batch, safe head origin is block A60
// - Unsafe head origin is A61
//
// Reorg L1 (start: block #61, depth: 22 blocks)
// - Rewind depth: Batch submission block + SeqWindowSize+1 blocks
// - Wind back to block #39
//
// Before building L2 to L1 head / syncing verifier & sequencer:
// Verifier
// - Unsafe head L1 origin is block #60
// - Safe head L1 origin is at genesis block #60
//
// Build Chain B
// - 62 blocks total
//   - 39 empty blocks left over from chain A
//   - 21 empty blocks
//   - empty block (61)
//   - empty block (62) <- Makes chain B longer than chain A, the re-org will be picked up
//
// After building L2 to L1 head:
// Verifier
// - Unsafe head is 62
// - Safe head is 42
func DeepReorg(gt *testing.T, deltaTimeOffset *hexutil.Uint64) {
	t := actionsHelpers.NewDefaultTesting(gt)

	// Create actor and verification engine client
	sd, dp, miner, sequencer, seqEngine, verifier, verifierEng, batcher := actionsHelpers.SetupReorgTest(t, &e2eutils.TestParams{
		MaxSequencerDrift:   40,
		SequencerWindowSize: 20,
		ChannelTimeout:      120,
		L1BlockTime:         4,
		AllocType:           config.AllocTypeStandard,
	}, deltaTimeOffset)
	minerCl := miner.L1Client(t, sd.RollupCfg)
	l2Client := seqEngine.EthClient()
	verifEngClient := verifierEng.EngineClient(t, sd.RollupCfg)
	checkVerifEngine := func() {
		ref, err := verifEngClient.L2BlockRefByLabel(t.Ctx(), eth.Safe)
		require.NoError(t, err)
		require.Equal(t, verifier.L2Safe(), ref, "verifier safe head of engine matches rollup client")
	}

	// Set up alice
	log := testlog.Logger(t, log.LevelDebug)
	addresses := e2eutils.CollectAddresses(sd, dp)
	l2UserEnv := &actionsHelpers.BasicUserEnv[*actionsHelpers.L2Bindings]{
		EthCl:          l2Client,
		Signer:         types.LatestSigner(sd.L2Cfg.Config),
		AddressCorpora: addresses,
		Bindings:       actionsHelpers.NewL2Bindings(t, l2Client, seqEngine.GethClient()),
	}
	alice := actionsHelpers.NewCrossLayerUser(log, dp.Secrets.Alice, rand.New(rand.NewSource(0xa57b)), config.AllocTypeStandard)
	alice.L2.SetUserEnv(l2UserEnv)

	// Run one iteration of the L2 derivation pipeline
	sequencer.ActL1HeadSignal(t)
	sequencer.ActL2PipelineFull(t)
	verifier.ActL2PipelineFull(t)

	// Start building chain A
	miner.ActL1SetFeeRecipient(common.Address{0x0A, 0x00})

	// Create a var to store the ref for the second to last block of the second sequencing window
	var blockA39 eth.L1BlockRef

	var aliceL2TxBlock *types.Block
	// Mine enough empty blocks on L1 to reach two sequence windows.
	for i := uint64(0); i < sd.RollupCfg.SeqWindowSize*3; i++ {
		// At block #50, send a batch to L1 containing all L2 blocks built up to this point.
		// This batch contains alice's transaction, and will be reorg'd out of the L1 chain
		// later in the test.
		if i == 50 {
			batcher.ActSubmitAll(t)

			miner.ActL1StartBlock(12)(t)
			miner.ActL1IncludeTx(sd.RollupCfg.Genesis.SystemConfig.BatcherAddr)(t)
			miner.ActL1EndBlock(t)
		} else {
			miner.ActEmptyBlock(t)
		}

		// Get the second to last block of the first sequence window
		// This is used later to verify the head of chain B after rewinding
		// chain A 1 sequence window + 1 block + Block A1 (batch submission with two
		// sequence windows worth of transactions)
		if i == sd.RollupCfg.SeqWindowSize*2-2 {
			var err error
			blockA39, err = minerCl.L1BlockRefByLabel(t.Ctx(), eth.Unsafe)
			require.NoError(t, err)
			require.Equal(t, uint64(39), blockA39.Number)
		}

		// Submit a dummy tx on L2 as alice with an L1 origin that will remain in the
		// canonical chain after the reorg. The batch that contains this transaction
		// will be submitted in block #50, which *will* be reorg'd out of the L1 chain, so the
		// L2 block that contains this transaction should no longer exist after L2 has
		// been re-derived from chain B later on in the test.
		if i == 35 {
			// Include alice's transaction on L2
			sequencer.ActL2StartBlock(t)

			// Submit a dummy tx
			alice.L2.ActResetTxOpts(t)
			alice.L2.ActSetTxToAddr(&dp.Addresses.Bob)(t)
			alice.L2.ActMakeTx(t)

			// Include the tx in the block we're making
			seqEngine.ActL2IncludeTx(alice.Address())(t)

			// Finalize the L2 block containing alice's transaction
			sequencer.ActL2EndBlock(t)

			// Store the ref to the L2 block that the transaction was included in for later.
			b0, err := l2Client.BlockByNumber(t.Ctx(), big.NewInt(int64(sequencer.L2Unsafe().Number)))
			require.NoError(t, err, "failed to fetch unsafe head of L2 after submitting alice's transaction")

			aliceL2TxBlock = b0
		}

		// Ask sequencer to handle new L1 head and build L2 blocks up to the L1 head
		sequencer.ActL1HeadSignal(t)
		sequencer.ActL2PipelineFull(t)
		sequencer.ActBuildToL1Head(t)
	}

	// Get the last empty block built in the loop above.
	// This will be the last block in the third sequencing window.
	blockA60, err := minerCl.L1BlockRefByLabel(t.Ctx(), eth.Unsafe)
	require.NoError(t, err)

	// Check that the safe head's L1 origin is block A50 before batch submission
	require.Equal(t, uint64(50), sequencer.L2Safe().L1Origin.Number)
	// Check that the unsafe head's L1 origin is block A60
	require.Equal(t, blockA60.ID(), sequencer.L2Unsafe().L1Origin)

	// Batch and submit all new L2 blocks that were built above to L1
	batcher.ActSubmitAll(t)

	// Build a new block on L1 that includes the L2 batch containing all blocks
	// between [51, 60]
	miner.ActL1SetFeeRecipient(common.Address{0x0A, 0x01})
	miner.ActL1StartBlock(12)(t)
	miner.ActL1IncludeTx(sd.RollupCfg.Genesis.SystemConfig.BatcherAddr)(t)
	miner.ActL1EndBlock(t)

	// Handle the new head block on both the verifier and the sequencer
	verifier.ActL1HeadSignal(t)
	sequencer.ActL1HeadSignal(t)

	// Run one iteration of the L2 derivation pipeline on both the verifier and sequencer
	verifier.ActL2PipelineFull(t)
	sequencer.ActL2PipelineFull(t)

	// Ensure that the verifier picks up that the L2 blocks were submitted to L1
	// and marks them as safe.
	// We check that the L2 safe L1 origin is block A240, or the last block
	// within the second sequencing window. This is the block directly before
	// the block that included the batch on chain A.
	require.Equal(t, blockA60.ID(), verifier.L2Safe().L1Origin)
	checkVerifEngine()

	// Perform a deep reorg the size of one sequencing window + 2 blocks.
	// This will affect the safe L2 chain.
	miner.ActL1RewindToParent(t)                              // Rewind the batch submission
	miner.ActL1RewindDepth(sd.RollupCfg.SeqWindowSize + 1)(t) // Rewind one sequence window + 1 block

	// Ensure that the block we rewinded to on L1 is the second to last block of the first
	// sequencing window.
	headAfterReorg, err := minerCl.L1BlockRefByLabel(t.Ctx(), eth.Unsafe)
	require.NoError(t, err)

	// Ensure that we landed on the intended L1 block after the reorg
	require.Equal(t, blockA39.ID(), headAfterReorg.ID())

	// Ensure that the safe L2 head has not been altered yet- we have not issued
	// a head signal to the sequencer or verifier post reorg.
	require.Equal(t, blockA60.ID(), verifier.L2Safe().L1Origin)
	require.Equal(t, blockA60.ID(), sequencer.L2Safe().L1Origin)
	// Ensure that the L2 unsafe head has not been altered yet- we have not issued
	// a head signal to the sequencer or verifier post reorg.
	require.Equal(t, blockA60.ID(), verifier.L2Unsafe().L1Origin)
	require.Equal(t, blockA60.ID(), sequencer.L2Unsafe().L1Origin)
	checkVerifEngine()

	// --------- [ CHAIN B ] ---------

	// Start building chain B
	miner.ActL1SetFeeRecipient(common.Address{0x0B, 0x00})
	// Mine enough empty blocks on L1 to reach three sequence windows or 60 blocks.
	// We already have 39 empty blocks on the rewinded L1 that are left over from chain A.
	for i := uint64(0); i < sd.RollupCfg.SeqWindowSize+1; i++ {
		miner.ActEmptyBlock(t)

		// Ask sequencer to handle new L1 head and build L2 blocks up to the L1 head
		sequencer.ActL1HeadSignal(t)
		sequencer.ActL2PipelineFull(t)
		sequencer.ActBuildToL1Head(t)
	}

	// Get the last unsafe block on chain B after creating SeqWindowSize+1 empty blocks
	blockB60, err := minerCl.L1BlockRefByLabel(t.Ctx(), eth.Unsafe)
	require.NoError(t, err)
	// Ensure blockB60 is #60 on chain B
	require.Equal(t, uint64(60), blockB60.Number)

	// Mine an empty block in place of the block that included the final batch on chain A
	miner.ActL1SetFeeRecipient(common.Address{0x0B, 0x01})
	miner.ActEmptyBlock(t)

	// Make block B62. the reorg is picked up when we have a new, longer chain.
	miner.ActL1SetFeeRecipient(common.Address{0x0B, 0x02})
	miner.ActEmptyBlock(t)
	blockB62, err := minerCl.L1BlockRefByLabel(t.Ctx(), eth.Unsafe)
	require.NoError(t, err)

	// Now sync the verifier. The batch from chain A is invalid, so it should have been ignored.
	// The safe head should have an origin at block B42
	verifier.ActL1HeadSignal(t)
	verifier.ActL2PipelineFull(t)

	// Ensure that the L2 Safe block is B42
	require.Equal(t, uint64(42), verifier.L2Safe().L1Origin.Number, "expected to be at block #42 after losing A40-61")
	require.NotZero(t, verifier.L2Safe().Number, "still preserving old L2 blocks that did not reference reorged L1 chain (assuming more than one L2 block per L1 block)")
	require.Equal(t, verifier.L2Safe(), verifier.L2Unsafe(), "L2 safe and unsafe head should be equal")
	checkVerifEngine()

	// Sync the sequencer, then build some new L2 blocks, up to and including with L1 origin B62
	sequencer.ActL1HeadSignal(t)
	sequencer.ActL2PipelineFull(t)
	sequencer.ActBuildToL1Head(t)
	require.Equal(t, sequencer.L2Unsafe().L1Origin, blockB62.ID())

	// Sync the verifier to the L2 chain with origin B62
	// Run an iteration of the derivation pipeline and ensure that the L2 safe L1 origin is block B62
	verifier.ActL1HeadSignal(t)
	verifier.ActL2PipelineFull(t)
	require.Equal(t, uint64(42), verifier.L2Safe().L1Origin.Number, "expected to be at block #42 after losing A40-61 and building 23 blocks on reorged chain")
	require.Equal(t, verifier.L2Safe(), verifier.L2Unsafe(), "L2 safe and unsafe head should be equal")
	checkVerifEngine()

	require.NotNil(gt, aliceL2TxBlock)

	// Ensure that the parent of the L2 block containing Alice's transaction still exists
	b0, err := l2Client.BlockByHash(t.Ctx(), aliceL2TxBlock.ParentHash())
	require.NoError(t, err, "Parent of the L2 block containing Alice's transaction should still exist on L2")
	require.Equal(t, b0.Hash(), aliceL2TxBlock.ParentHash())

	// Ensure that the L2 block containing Alice's transaction no longer exists.
	b1, err := l2Client.BlockByNumber(t.Ctx(), aliceL2TxBlock.Number())
	require.NoError(t, err, "A block that has the same number as the block that contained Alice's transaction should still exist on L2")
	require.Equal(t, b1.Number(), aliceL2TxBlock.Number())
	require.NotEqual(t, b1.Hash(), aliceL2TxBlock.Hash(), "L2 block containing Alice's transaction should no longer exist on L2")
}

type rpcWrapper struct {
	client.RPC
}

// RestartOpGeth tests that the sequencer can restart its execution engine without rollup-node restart,
// including recovering the finalized/safe state of L2 chain without reorging.
func RestartOpGeth(gt *testing.T, deltaTimeOffset *hexutil.Uint64) {
	t := actionsHelpers.NewDefaultTesting(gt)
	dbPath := path.Join(t.TempDir(), "testdb")
	dbOption := func(_ *ethconfig.Config, nodeCfg *node.Config) error {
		nodeCfg.DataDir = dbPath
		return nil
	}
	dp := e2eutils.MakeDeployParams(t, actionsHelpers.DefaultRollupTestParams())
	upgradesHelpers.ApplyDeltaTimeOffset(dp, deltaTimeOffset)
	sd := e2eutils.Setup(t, dp, actionsHelpers.DefaultAlloc)
	log := testlog.Logger(t, log.LevelDebug)
	jwtPath := e2eutils.WriteDefaultJWT(t)
	// L1
	miner := actionsHelpers.NewL1Miner(t, log, sd.L1Cfg)
	l1F, err := sources.NewL1Client(miner.RPCClient(), log, nil, sources.L1ClientDefaultConfig(sd.RollupCfg, false, sources.RPCKindStandard))
	require.NoError(t, err)
	// Sequencer
	seqEng := actionsHelpers.NewL2Engine(t, log, sd.L2Cfg, jwtPath, dbOption)
	engRpc := &rpcWrapper{seqEng.RPCClient()}
	l2Cl, err := sources.NewEngineClient(engRpc, log, nil, sources.EngineClientDefaultConfig(sd.RollupCfg))
	require.NoError(t, err)
	sequencer := actionsHelpers.NewL2Sequencer(t, log, l1F, miner.BlobStore(), altda.Disabled, l2Cl, sd.RollupCfg, 0, nil)

	batcher := actionsHelpers.NewL2Batcher(log, sd.RollupCfg, actionsHelpers.DefaultBatcherCfg(dp),
		sequencer.RollupClient(), miner.EthClient(), seqEng.EthClient(), seqEng.EngineClient(t, sd.RollupCfg))

	// start
	sequencer.ActL2PipelineFull(t)

	miner.ActEmptyBlock(t)

	buildAndSubmit := func() {
		// build some blocks
		sequencer.ActL1HeadSignal(t)
		sequencer.ActBuildToL1Head(t)
		// submit the blocks, confirm on L1
		batcher.ActSubmitAll(t)
		miner.ActL1StartBlock(12)(t)
		miner.ActL1IncludeTx(sd.RollupCfg.Genesis.SystemConfig.BatcherAddr)(t)
		miner.ActL1EndBlock(t)
		sequencer.ActL2PipelineFull(t)
	}
	buildAndSubmit()

	// finalize the L1 data (first block, and the new block with batch)
	miner.ActL1SafeNext(t)
	miner.ActL1SafeNext(t)
	miner.ActL1FinalizeNext(t)
	miner.ActL1FinalizeNext(t)
	sequencer.ActL1FinalizedSignal(t)
	sequencer.ActL1SafeSignal(t)

	// build and submit more
	buildAndSubmit()
	// but only mark the L1 block with this batch as safe
	miner.ActL1SafeNext(t)
	sequencer.ActL1SafeSignal(t)

	// build some more, these stay unsafe
	miner.ActEmptyBlock(t)
	sequencer.ActL1HeadSignal(t)
	sequencer.ActBuildToL1Head(t)

	statusBeforeRestart := sequencer.SyncStatus()
	// before restart scenario: we have a distinct finalized, safe, and unsafe part of the L2 chain
	require.NotZero(t, statusBeforeRestart.FinalizedL2.L1Origin.Number)
	require.Less(t, statusBeforeRestart.FinalizedL2.L1Origin.Number, statusBeforeRestart.SafeL2.L1Origin.Number)
	require.Less(t, statusBeforeRestart.SafeL2.L1Origin.Number, statusBeforeRestart.UnsafeL2.L1Origin.Number)

	// close the sequencer engine
	require.NoError(t, seqEng.Close())
	// and start a new one with same db path
	seqEngNew := actionsHelpers.NewL2Engine(t, log, sd.L2Cfg, jwtPath, dbOption)
	// swap in the new rpc. This is as close as we can get to reconnecting to a new in-memory rpc connection
	engRpc.RPC = seqEngNew.RPCClient()

	// note: geth does not persist the safe block label, only the finalized block label
	safe, err := l2Cl.L2BlockRefByLabel(t.Ctx(), eth.Safe)
	require.NoError(t, err)
	finalized, err := l2Cl.L2BlockRefByLabel(t.Ctx(), eth.Finalized)
	require.NoError(t, err)
	require.Equal(t, statusBeforeRestart.FinalizedL2, safe, "expecting to revert safe head to finalized head upon restart")
	require.Equal(t, statusBeforeRestart.FinalizedL2, finalized, "expecting to keep same finalized head upon restart")

	// sequencer runs pipeline, but now attached to the restarted geth node
	sequencer.ActL2PipelineFull(t)
	require.Equal(t, statusBeforeRestart.UnsafeL2, sequencer.L2Unsafe(), "expecting to keep same unsafe head upon restart")
	require.Equal(t, statusBeforeRestart.SafeL2, sequencer.L2Safe(), "expecting the safe block to catch up to what it was before shutdown after syncing from L1, and not be stuck at the finalized block")
}

// ConflictingL2Blocks tests that a second copy of the sequencer stack cannot introduce an alternative
// L2 block (compared to something already secured by the first sequencer):
// the alt block is not synced by the verifier, in unsafe and safe sync modes.
func ConflictingL2Blocks(gt *testing.T, deltaTimeOffset *hexutil.Uint64) {
	t := actionsHelpers.NewDefaultTesting(gt)
	dp := e2eutils.MakeDeployParams(t, actionsHelpers.DefaultRollupTestParams())
	upgradesHelpers.ApplyDeltaTimeOffset(dp, deltaTimeOffset)
	sd := e2eutils.Setup(t, dp, actionsHelpers.DefaultAlloc)
	log := testlog.Logger(t, log.LevelDebug)

	sd, _, miner, sequencer, seqEng, verifier, _, batcher := actionsHelpers.SetupReorgTestActors(t, dp, sd, log)

	// Extra setup: a full alternative sequencer, sequencer engine, and batcher
	jwtPath := e2eutils.WriteDefaultJWT(t)
	altSeqEng := actionsHelpers.NewL2Engine(t, log, sd.L2Cfg, jwtPath)
	altSeqEngCl, err := sources.NewEngineClient(altSeqEng.RPCClient(), log, nil, sources.EngineClientDefaultConfig(sd.RollupCfg))
	require.NoError(t, err)
	l1F, err := sources.NewL1Client(miner.RPCClient(), log, nil, sources.L1ClientDefaultConfig(sd.RollupCfg, false, sources.RPCKindStandard))
	require.NoError(t, err)
	altSequencer := actionsHelpers.NewL2Sequencer(t, log, l1F, miner.BlobStore(), altda.Disabled, altSeqEngCl, sd.RollupCfg, 0, nil)
	altBatcher := actionsHelpers.NewL2Batcher(log, sd.RollupCfg, actionsHelpers.DefaultBatcherCfg(dp),
		altSequencer.RollupClient(), miner.EthClient(), altSeqEng.EthClient(), altSeqEng.EngineClient(t, sd.RollupCfg))

	// And set up user Alice, using the alternative sequencer endpoint
	l2Cl := altSeqEng.EthClient()
	addresses := e2eutils.CollectAddresses(sd, dp)
	l2UserEnv := &actionsHelpers.BasicUserEnv[*actionsHelpers.L2Bindings]{
		EthCl:          l2Cl,
		Signer:         types.LatestSigner(sd.L2Cfg.Config),
		AddressCorpora: addresses,
		Bindings:       actionsHelpers.NewL2Bindings(t, l2Cl, altSeqEng.GethClient()),
	}
	alice := actionsHelpers.NewCrossLayerUser(log, dp.Secrets.Alice, rand.New(rand.NewSource(1234)), config.AllocTypeStandard)
	alice.L2.SetUserEnv(l2UserEnv)

	sequencer.ActL2PipelineFull(t)
	verifier.ActL2PipelineFull(t)
	altSequencer.ActL2PipelineFull(t)

	// build empty L1 block
	miner.ActEmptyBlock(t)

	// Create L2 blocks, and reference the L1 head as origin
	sequencer.ActL1HeadSignal(t)
	sequencer.ActBuildToL1Head(t)

	// submit all new L2 blocks
	batcher.ActSubmitAll(t)

	// new L1 block with L2 batch
	miner.ActL1StartBlock(12)(t)
	miner.ActL1IncludeTx(sd.RollupCfg.Genesis.SystemConfig.BatcherAddr)(t)
	miner.ActL1EndBlock(t)

	// verifier picks up the L2 chain that was submitted
	verifier.ActL1HeadSignal(t)
	verifier.ActL2PipelineFull(t)
	verifierHead := verifier.L2Unsafe()
	require.Equal(t, verifier.L2Safe(), sequencer.L2Unsafe(), "verifier syncs from sequencer via L1")
	require.Equal(t, verifier.L2Safe(), verifierHead, "verifier head is the same as that what was derived from L1")
	require.NotEqual(t, sequencer.L2Safe(), sequencer.L2Unsafe(), "sequencer has not processed L1 yet")

	require.Less(t, altSequencer.L2Unsafe().L1Origin.Number, sequencer.L2Unsafe().L1Origin.Number, "alt-sequencer is behind")

	// produce a conflicting L2 block with the alt sequencer:
	// a new unsafe block that should not replace the existing safe block at the same height
	altSequencer.ActL2StartBlock(t)
	// include tx to force the L2 block to really be different than the previous empty block
	alice.L2.ActResetTxOpts(t)
	alice.L2.ActSetTxToAddr(&dp.Addresses.Bob)(t)
	alice.L2.ActMakeTx(t)
	altSeqEng.ActL2IncludeTx(alice.Address())(t)
	altSequencer.ActL2EndBlock(t)

	conflictBlock := seqEng.L2Chain().GetBlockByNumber(altSequencer.L2Unsafe().Number)
	require.NotEqual(t, conflictBlock.Hash(), altSequencer.L2Unsafe().Hash, "alt sequencer has built a conflicting block")

	// give the unsafe block to the verifier, and see if it reorgs because of any unsafe inputs
	head, err := altSeqEngCl.PayloadByLabel(t.Ctx(), eth.Unsafe)
	require.NoError(t, err)
	verifier.ActL2UnsafeGossipReceive(head)(t)

	// make sure verifier has processed everything
	verifier.ActL2PipelineFull(t)

	// check if verifier is still following safe chain
	require.Equal(t, verifier.L2Unsafe(), verifierHead, "verifier must not accept the unsafe payload that orphans the safe payload")

	// now submit it to L1, and see if the verifier respects the inclusion order and preserves the original block
	altBatcher.ActSubmitAll(t)
	// include it in L1
	miner.ActL1StartBlock(12)(t)
	miner.ActL1IncludeTx(sd.RollupCfg.Genesis.SystemConfig.BatcherAddr)(t)
	miner.ActL1EndBlock(t)
	l1Number := miner.L1Chain().CurrentHeader().Number.Uint64()

	// show latest L1 block with new batch data to verifier, and make it sync.
	verifier.ActL1HeadSignal(t)
	verifier.ActL2PipelineFull(t)
	require.Equal(t, verifier.SyncStatus().CurrentL1.Number, l1Number, "verifier has synced all new L1 blocks")
	require.Equal(t, verifier.L2Unsafe(), verifierHead, "verifier sticks to first included L2 block")

	// Now make the alt sequencer aware of the L1 chain and derive the L2 chain like the verifier;
	// it should reorg out its conflicting blocks to get back in harmony with the verifier.
	altSequencer.ActL1HeadSignal(t)
	altSequencer.ActL2PipelineFull(t)
	require.Equal(t, verifier.L2Unsafe(), altSequencer.L2Unsafe(), "alt-sequencer gets back in harmony with verifier by reorging out its conflicting data")
	require.Equal(t, sequencer.L2Unsafe(), altSequencer.L2Unsafe(), "and gets back in harmony with original sequencer")
}

func SyncAfterReorg(gt *testing.T, deltaTimeOffset *hexutil.Uint64) {
	t := actionsHelpers.NewDefaultTesting(gt)
	testingParams := e2eutils.TestParams{
		MaxSequencerDrift:   60,
		SequencerWindowSize: 4,
		ChannelTimeout:      2,
		L1BlockTime:         12,
		AllocType:           config.AllocTypeStandard,
	}
	sd, dp, miner, sequencer, seqEngine, verifier, _, batcher := actionsHelpers.SetupReorgTest(t, &testingParams, deltaTimeOffset)
	l2Client := seqEngine.EthClient()
	log := testlog.Logger(t, log.LevelDebug)
	addresses := e2eutils.CollectAddresses(sd, dp)
	l2UserEnv := &actionsHelpers.BasicUserEnv[*actionsHelpers.L2Bindings]{
		EthCl:          l2Client,
		Signer:         types.LatestSigner(sd.L2Cfg.Config),
		AddressCorpora: addresses,
		Bindings:       actionsHelpers.NewL2Bindings(t, l2Client, seqEngine.GethClient()),
	}
	alice := actionsHelpers.NewCrossLayerUser(log, dp.Secrets.Alice, rand.New(rand.NewSource(0xa57b)), config.AllocTypeStandard)
	alice.L2.SetUserEnv(l2UserEnv)

	sequencer.ActL2PipelineFull(t)
	verifier.ActL2PipelineFull(t)

	// build empty L1 block: A0
	miner.ActL1SetFeeRecipient(common.Address{'A', 0})
	miner.ActEmptyBlock(t)
	sequencer.ActL1HeadSignal(t)
	for sequencer.L2Unsafe().L1Origin.Number < sequencer.SyncStatus().HeadL1.Number {
		// build L2 blocks until the L1 origin is the current L1 head(A0)
		sequencer.ActL2PipelineFull(t)
		sequencer.ActL2StartBlock(t)
		if sequencer.L2Unsafe().Number == 11 {
			// include a user tx at L2 block #12 to make a state transition
			alice.L2.ActResetTxOpts(t)
			alice.L2.ActSetTxToAddr(&dp.Addresses.Bob)(t)
			alice.L2.ActMakeTx(t)
			// Include the tx in the block we're making
			seqEngine.ActL2IncludeTx(alice.Address())(t)
		}
		sequencer.ActL2EndBlock(t)
	}
	// submit all new L2 blocks: #1 ~ #12
	batcher.ActSubmitAll(t)

	// build an L1 block included batch TX: A1
	miner.ActL1SetFeeRecipient(common.Address{'A', 1})
	miner.ActL1StartBlock(12)(t)
	miner.ActL1IncludeTx(sd.RollupCfg.Genesis.SystemConfig.BatcherAddr)(t)
	miner.ActL1EndBlock(t)

	for i := 2; i < 6; i++ {
		// build L2 blocks until the L1 origin is the current L1 head
		sequencer.ActL1HeadSignal(t)
		sequencer.ActBuildToL1Head(t)
		// submt all new L2 blocks
		batcher.ActSubmitAll(t)

		// build an L1 block included batch TX: A2 ~ A5
		miner.ActL1SetFeeRecipient(common.Address{'A', byte(i)})
		miner.ActL1StartBlock(12)(t)
		miner.ActL1IncludeTx(sd.RollupCfg.Genesis.SystemConfig.BatcherAddr)(t)
		miner.ActL1EndBlock(t)
	}

	sequencer.ActL1HeadSignal(t)
	sequencer.ActL2PipelineFull(t)
	// capture current L2 safe head
	submittedSafeHead := sequencer.L2Safe().ID()

	// build L2 blocks until the L1 origin is the current L1 head(A5)
	sequencer.ActBuildToL1Head(t)
	batcher.ActSubmitAll(t)

	// build an L1 block included batch TX: A6
	miner.ActL1SetFeeRecipient(common.Address{'A', 6})
	miner.ActL1StartBlock(12)(t)
	miner.ActL1IncludeTx(sd.RollupCfg.Genesis.SystemConfig.BatcherAddr)(t)
	miner.ActL1EndBlock(t)

	sequencer.ActL1HeadSignal(t)
	sequencer.ActL2PipelineFull(t)
	verifier.ActL1HeadSignal(t)
	verifier.ActL2PipelineFull(t)

	// reorg L1
	miner.ActL1RewindToParent(t)                       // undo A6
	miner.ActL1SetFeeRecipient(common.Address{'B', 6}) // build B6
	miner.ActEmptyBlock(t)
	miner.ActL1SetFeeRecipient(common.Address{'B', 7}) // build B7
	miner.ActEmptyBlock(t)

	// sequencer and verifier detect L1 reorg
	// derivation pipeline is reset
	// safe head may be reset to block #11
	sequencer.ActL1HeadSignal(t)
	sequencer.ActL2PipelineFull(t)
	verifier.ActL1HeadSignal(t)
	verifier.ActL2PipelineFull(t)

	// sequencer and verifier must derive all submitted batches and reach to the captured block
	require.Equal(t, sequencer.L2Safe().ID(), submittedSafeHead)
	require.Equal(t, verifier.L2Safe().ID(), submittedSafeHead)
}
