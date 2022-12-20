package actions

import (
	"path"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils"
	"github.com/ethereum-optimism/optimism/op-node/client"
	"github.com/ethereum-optimism/optimism/op-node/eth"
	"github.com/ethereum-optimism/optimism/op-node/sources"
	"github.com/ethereum-optimism/optimism/op-node/testlog"
)

func setupReorgTest(t Testing) (*e2eutils.SetupData, *L1Miner, *L2Sequencer, *L2Engine, *L2Verifier, *L2Engine, *L2Batcher) {
	dp := e2eutils.MakeDeployParams(t, defaultRollupTestParams)
	sd := e2eutils.Setup(t, dp, defaultAlloc)
	log := testlog.Logger(t, log.LvlDebug)
	miner, seqEngine, sequencer := setupSequencerTest(t, sd, log)
	miner.ActL1SetFeeRecipient(common.Address{'A'})
	sequencer.ActL2PipelineFull(t)
	verifEngine, verifier := setupVerifier(t, sd, log, miner.L1Client(t, sd.RollupCfg))
	rollupSeqCl := sequencer.RollupClient()
	batcher := NewL2Batcher(log, sd.RollupCfg, &BatcherCfg{
		MinL1TxSize: 0,
		MaxL1TxSize: 128_000,
		BatcherKey:  dp.Secrets.Batcher,
	}, rollupSeqCl, miner.EthClient(), seqEngine.EthClient())
	return sd, miner, sequencer, seqEngine, verifier, verifEngine, batcher
}

func TestReorgOrphanBlock(gt *testing.T) {
	t := NewDefaultTesting(gt)
	sd, miner, sequencer, _, verifier, verifierEng, batcher := setupReorgTest(t)
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
	batchTx := miner.l1Transactions[0]
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
	require.NoError(t, miner.eth.TxPool().AddLocal(batchTx))
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

func TestReorgFlipFlop(gt *testing.T) {
	t := NewDefaultTesting(gt)
	sd, miner, sequencer, _, verifier, verifierEng, batcher := setupReorgTest(t)
	minerCl := miner.L1Client(t, sd.RollupCfg)
	verifEngClient := verifierEng.EngineClient(t, sd.RollupCfg)
	checkVerifEngine := func() {
		// TODO: geth preserves L2 chain with origin A1 after flip-flopping to B?
		//ref, err := verifEngClient.L2BlockRefByLabel(t.Ctx(), eth.Unsafe)
		//require.NoError(t, err)
		//t.Logf("l2 unsafe head %s with origin %s", ref, ref.L1Origin)
		//require.NotEqual(t, verifier.L2Unsafe().Hash, ref.ParentHash, "TODO off by one, engine syncs A0 after reorging back from B, while rollup node only inserts up to A0 (excl.)")
		//require.Equal(t, verifier.L2Unsafe(), ref, "verifier safe head of engine matches rollup client")

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
	batchTxA := miner.l1Transactions[0]
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
	require.NoError(t, miner.eth.TxPool().AddLocal(batchTxA))
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

	require.NotZero(t, verifier.L2Safe().Number, "still preserving old L2 blocks that did not reference reorged L1 chain (assuming more than one L2 block per L1 block)")
	require.Equal(t, verifier.L2Safe(), verifier.L2Unsafe(), "head is at safe block after L1 reorg")
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
	require.NoError(t, miner.eth.TxPool().AddLocal(batchTxA)) // replay chain A batches, but now in A2 instead of A1
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
// 1. Create an L1 actor
// 2. Ask the L1 actor to build two sequence windows of empty blocks
// 3. Ask the L2 sequencer to build a chain that references these L1 blocks
// 4. Ask the batch submitter to submit to L1
// 5. Ask the L1 to include this data
// 6. If the verifier syncs, they’ll be marked as safe
// 7. ...
//
// Chain A
// - 241 blocks total
//   - 240 blank
//   - 241 includes batch with blocks [1, 240]
//
// Verifier
// - Prior to batch submission, safe head origin is block 120
// - After batch, safe head origin is block 240
// - Unsafe head origin is 241
//
// Reorg L1 (start: block #240, depth: 122 blocks)
// - Rewind depth: Batch submission block + SeqWindowSize+1 blocks
// - Wind back to block #119 on chain A
//
// Before building L2 to L1 head:
// Verifier
// - Unsafe head L1 origin is 119
// - Safe head L1 origin is at genesis (?)
//   - Actual: Safe head is at block #123
//
// Build Chain B
// - 242 blocks total
//   - 119 blocks left over from chain A
//   - 121 empty blocks
//   - batch block (241)
//   - longer block (242)
//
// After building L2 to L1 head:
// Verifier (?)
// - Unsafe head is 242 (?)
// - Safe head is 240 (?)
func TestDeepReorg(gt *testing.T) {
	t := NewDefaultTesting(gt)

	// Create actor and verification engine client
	sd, miner, sequencer, _, verifier, verifierEng, batcher := setupReorgTest(t)
	minerCl := miner.L1Client(t, sd.RollupCfg)
	verifEngClient := verifierEng.EngineClient(t, sd.RollupCfg)
	checkVerifEngine := func() {
		ref, err := verifEngClient.L2BlockRefByLabel(t.Ctx(), eth.Safe)
		require.NoError(t, err)
		require.Equal(t, verifier.L2Safe(), ref, "verifier safe head of engine matches rollup client")
	}

	// Run one iteration of the L2 derivation pipeline
	sequencer.ActL2PipelineFull(t)
	verifier.ActL2PipelineFull(t)

	// TODO: Remove, just for sanity checking
	require.Equal(t, sd.RollupCfg.SeqWindowSize, uint64(120))

	// Start building chain A
	miner.ActL1SetFeeRecipient(common.Address{0x0A, 0x00})

	require.Equal(t, uint64(0), sequencer.L2Safe().L1Origin.Number)

	// Create a var for the second to last block ref of the first sequencing window
	var blockA119 eth.L1BlockRef

	// Mine enough empty blocks on L1 to reach two sequence windows.
	for i := uint64(0); i < sd.RollupCfg.SeqWindowSize*2; i++ {
		miner.ActEmptyBlock(t)

		// Get the second to last block of the first sequence window
		// This is used later to verify the head of chain B after rewinding
		// chain A 1 sequence window + 1 block + Block A1 (batch submission with two
		// sequence windows worth of transactions)
		if i == sd.RollupCfg.SeqWindowSize-2 {
			var err error
			blockA119, err = minerCl.L1BlockRefByLabel(t.Ctx(), eth.Unsafe)
			require.NoError(t, err)
		}
	}

	// Get the last empty block built in the loop above.
	// This will be the last block in the second sequencing window.
	blockA240, err := minerCl.L1BlockRefByLabel(t.Ctx(), eth.Unsafe)
	require.NoError(t, err)

	// Create L2 blocks and reference the L1 head as the origin
	sequencer.ActL1HeadSignal(t)
	sequencer.ActBuildToL1Head(t)

	// Check that the safe head's L1 origin is block #121 before batch submission
	// Question: Shouldn't the safe head be @ #120?
	require.Equal(t, uint64(121), sequencer.L2Safe().L1Origin.Number)
	// Check that the unsafe head's L1 origin is block #240
	require.Equal(t, blockA240.ID(), sequencer.L2Unsafe().L1Origin)

	// Batch and submit all new L2 blocks that were built above to L1
	batcher.ActSubmitAll(t)

	// Build a new block on L1 that includes the L2 batch
	miner.ActL1SetFeeRecipient(common.Address{0x0A, 0x01})
	miner.ActL1StartBlock(12)(t)
	miner.ActL1IncludeTx(sd.RollupCfg.Genesis.SystemConfig.BatcherAddr)(t)
	batchTxA := miner.l1Transactions[0]
	miner.ActL1EndBlock(t)

	// Run one iteration of the L2 derivation pipeline
	verifier.ActL1HeadSignal(t)
	verifier.ActL2PipelineFull(t)

	// Ensure that the verifier picks up that the L2 blocks were submitted to L1
	// and marks them as safe.
	// We check that the L2 safe L1 origin is block A240, or the last block
	// within the second sequencing window. This is the block directly before
	// the block that included the batch on chain A.
	require.Equal(t, blockA240.ID(), verifier.L2Safe().L1Origin)
	checkVerifEngine()

	// Perform a deep reorg the size of one sequencing window.
	// This will affect the safe L2 chain.
	miner.ActL1RewindToParent(t)                              // Rewind the batch submission
	miner.ActL1RewindDepth(sd.RollupCfg.SeqWindowSize + 1)(t) // Rewind one sequence window + 1 block

	// Ensure that the block we rewinded to on L1 is the second to last block of the first
	// sequencing window.
	headAfterReorg, err := minerCl.L1BlockRefByLabel(t.Ctx(), eth.Unsafe)
	require.NoError(t, err)
	require.Equal(t, blockA119.ID(), headAfterReorg.ID())

	// Ensure that the safe L2 chain has not been altered yet.
	require.Equal(t, blockA240.ID(), verifier.L2Safe().L1Origin)

	// --------- [ CHAIN B ] ---------

	// Start building chain B
	miner.ActL1SetFeeRecipient(common.Address{0x0B, 0x00})
	// Mine enough empty blocks on L1 to reach two sequence windows or 240 blocks.
	// We already have 119 empty blocks on the rewinded chain.
	for i := uint64(0); i < sd.RollupCfg.SeqWindowSize+1; i++ {
		miner.ActEmptyBlock(t)
	}

	// Get the last unsafe block on chain B after creating SeqWindowSize+1 empty blocks
	blockB240, err := minerCl.L1BlockRefByLabel(t.Ctx(), eth.Unsafe)
	require.NoError(t, err)
	// Ensure blockB240 is #240 on chain B
	require.Equal(t, uint64(240), blockB240.Number)

	// Re-include the batch that included two sequence windows worth of transactions
	// that was built on chain A
	// TODO: This batch should be invalid (?) - Josh
	miner.ActL1SetFeeRecipient(common.Address{0x0B, 0x01})
	miner.ActL1StartBlock(12)(t)
	require.NoError(t, miner.eth.TxPool().AddLocal(batchTxA))
	miner.ActL1IncludeTx(sd.RollupCfg.Genesis.SystemConfig.BatcherAddr)(t)
	miner.ActL1EndBlock(t)

	// make B2, the reorg is picked up when we have a new, longer chain.
	// This will be block #242 on chain B.
	miner.ActL1SetFeeRecipient(common.Address{0x0B, 0x02})
	miner.ActEmptyBlock(t)
	blockB2, err := minerCl.L1BlockRefByLabel(t.Ctx(), eth.Unsafe)
	require.NoError(t, err)

	// -- slightly modified logic from TestReorgFlipFlop. Logic below is likely incorrect. --
	// Now sync the verifier. Some of the batches should be ignored.
	// The safe head should have an origin at block B240 (?)
	// verifier.ActL1HeadSignal(t)
	// verifier.ActL2PipelineFull(t)
	sequencer.ActL1HeadSignal(t)
	sequencer.ActBuildToL1Head(t)
	sequencer.ActL2PipelineFull(t)
	// require.Equal(t, blockB240.ID(), verifier.L2Safe().L1Origin, "expected to be back at genesis origin after losing A0 and A1")

	require.Equal(t, uint64(242), sequencer.L2Unsafe().L1Origin.Number)
	require.NotZero(t, verifier.L2Safe().Number, "still preserving old L2 blocks that did not reference reorged L1 chain (assuming more than one L2 block per L1 block)")
	require.Equal(t, verifier.L2Safe(), verifier.L2Unsafe(), "head is at safe block after L1 reorg")
	checkVerifEngine()

	// and sync the sequencer, then build some new L2 blocks, up to and including with L1 origin B2
	sequencer.ActL1HeadSignal(t)
	sequencer.ActL2PipelineFull(t)
	sequencer.ActBuildToL1Head(t)
	require.Equal(t, sequencer.L2Unsafe().L1Origin, blockB2.ID(), "B2 is the unsafe L1 origin of sequencer now")
	// -- snip --
}

type rpcWrapper struct {
	client.RPC
}

// TestRestartOpGeth tests that the sequencer can restart its execution engine without rollup-node restart,
// including recovering the finalized/safe state of L2 chain without reorging.
func TestRestartOpGeth(gt *testing.T) {
	t := NewDefaultTesting(gt)
	dbPath := path.Join(t.TempDir(), "testdb")
	dbOption := func(_ *ethconfig.Config, nodeCfg *node.Config) error {
		nodeCfg.DataDir = dbPath
		return nil
	}
	dp := e2eutils.MakeDeployParams(t, defaultRollupTestParams)
	sd := e2eutils.Setup(t, dp, defaultAlloc)
	log := testlog.Logger(t, log.LvlDebug)
	jwtPath := e2eutils.WriteDefaultJWT(t)
	// L1
	miner := NewL1Miner(t, log, sd.L1Cfg)
	l1F, err := sources.NewL1Client(miner.RPCClient(), log, nil, sources.L1ClientDefaultConfig(sd.RollupCfg, false))
	require.NoError(t, err)
	// Sequencer
	seqEng := NewL2Engine(t, log, sd.L2Cfg, sd.RollupCfg.Genesis.L1, jwtPath, dbOption)
	engRpc := &rpcWrapper{seqEng.RPCClient()}
	l2Cl, err := sources.NewEngineClient(engRpc, log, nil, sources.EngineClientDefaultConfig(sd.RollupCfg))
	require.NoError(t, err)
	sequencer := NewL2Sequencer(t, log, l1F, l2Cl, sd.RollupCfg, 0)

	batcher := NewL2Batcher(log, sd.RollupCfg, &BatcherCfg{
		MinL1TxSize: 0,
		MaxL1TxSize: 128_000,
		BatcherKey:  dp.Secrets.Batcher,
	}, sequencer.RollupClient(), miner.EthClient(), seqEng.EthClient())

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
	seqEngNew := NewL2Engine(t, log, sd.L2Cfg, sd.RollupCfg.Genesis.L1, jwtPath, dbOption)
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
