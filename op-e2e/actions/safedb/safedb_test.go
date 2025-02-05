package safedb

import (
	"context"
	"testing"

	actionsHelpers "github.com/ethereum-optimism/optimism/op-e2e/actions/helpers"
	"github.com/ethereum-optimism/optimism/op-e2e/actions/safedb/helpers"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
)

func TestRecordSafeHeadUpdates(gt *testing.T) {
	t := actionsHelpers.NewDefaultTesting(gt)
	sd, miner, sequencer, verifier, verifierEng, batcher := helpers.SetupSafeDBTest(t, actionsHelpers.DefaultRollupTestParams())
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

	// Verify the safe head is recorded
	l1Head := miner.L1Chain().CurrentBlock()
	firstSafeHeadUpdateL1Block := l1Head.Number.Uint64()
	response, err := verifier.RollupClient().SafeHeadAtL1Block(context.Background(), firstSafeHeadUpdateL1Block)
	require.NoError(t, err)
	require.Equal(t, eth.HeaderBlockID(l1Head), response.L1Block)
	require.Equal(t, verifier.L2Unsafe().ID(), response.SafeHead)

	// Should get the same result for anything after that L1 block too
	response, err = verifier.RollupClient().SafeHeadAtL1Block(context.Background(), firstSafeHeadUpdateL1Block+1)
	require.NoError(t, err)
	require.Equal(t, eth.HeaderBlockID(l1Head), response.L1Block)
	require.Equal(t, verifier.L2Unsafe().ID(), response.SafeHead)

	// Only genesis is safe at this point
	response, err = verifier.RollupClient().SafeHeadAtL1Block(context.Background(), firstSafeHeadUpdateL1Block-1)
	require.NoError(t, err)
	require.Equal(t, eth.HeaderBlockID(miner.L1Chain().Genesis().Header()), response.L1Block)
	require.Equal(t, sd.RollupCfg.Genesis.L2, response.SafeHead)

	// orphan the L1 block that included the batch tx, and build a new different L1 block
	miner.ActL1RewindToParent(t)
	miner.ActL1SetFeeRecipient(common.Address{'B'})
	miner.ActEmptyBlock(t)
	miner.ActEmptyBlock(t) // needs to be a longer chain for reorg to be applied.

	// sync verifier again. The L1 reorg excluded the batch, so now the previous L2 chain should be unsafe again.
	// However, the L2 chain can still be canonical later, since it did not reference the reorged L1 block
	verifier.ActL1HeadSignal(t)
	verifier.ActL2PipelineFull(t)
	require.Equal(t, verifier.L2Safe(), sequencer.L2Safe(), "verifier rewinds safe when L1 reorgs out batch")
	ref, err := verifEngClient.L2BlockRefByLabel(t.Ctx(), eth.Safe)
	require.NoError(t, err)
	require.Equal(t, verifier.L2Safe(), ref, "verifier engine matches rollup client")

	// The safe head has been reorged so the record should have been deleted, leaving us back with just genesis safe
	response, err = verifier.RollupClient().SafeHeadAtL1Block(context.Background(), firstSafeHeadUpdateL1Block)
	require.NoError(t, err)
	require.Equal(t, eth.HeaderBlockID(miner.L1Chain().Genesis().Header()), response.L1Block)
	require.Equal(t, sd.RollupCfg.Genesis.L2, response.SafeHead)

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

	// Verify the safe head is recorded again
	l1Head = miner.L1Chain().CurrentBlock()
	firstSafeHeadUpdateL1Block = l1Head.Number.Uint64()
	response, err = verifier.RollupClient().SafeHeadAtL1Block(context.Background(), firstSafeHeadUpdateL1Block)
	require.NoError(t, err)
	require.Equal(t, eth.HeaderBlockID(l1Head), response.L1Block)
	require.Equal(t, verifier.L2Unsafe().ID(), response.SafeHead)
}
