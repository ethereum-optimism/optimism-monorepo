package proofs

import (
	"testing"

	actionsHelpers "github.com/ethereum-optimism/optimism/op-e2e/actions/helpers"
	"github.com/ethereum-optimism/optimism/op-e2e/actions/proofs/helpers"
	"github.com/ethereum-optimism/optimism/op-program/client/claim"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
)

func runBadTxInBatchTest(gt *testing.T, testCfg *helpers.TestCfg[any]) {
	t := actionsHelpers.NewDefaultTesting(gt)
	env := helpers.NewL2FaultProofEnv(t, testCfg, helpers.NewTestParams(), helpers.NewBatcherCfg())

	// Build a block on L2 with 1 tx.
	env.Alice.L2.ActResetTxOpts(t)
	env.Alice.L2.ActSetTxToAddr(&env.Dp.Addresses.Bob)
	env.Alice.L2.ActMakeTx(t)

	env.Sequencer.ActL2StartBlock(t)
	env.Engine.ActL2IncludeTx(env.Alice.Address())(t)
	env.Sequencer.ActL2EndBlock(t)
	env.Alice.L2.ActCheckReceiptStatusOfLastTx(true)(t)

	// Instruct the batcher to submit a faulty channel, with an invalid tx.
	env.Batcher.ActL2BatchBuffer(t, func(block *types.Block) *types.Block {
		// Replace the tx with one that has a bad signature.
		txs := block.Transactions()
		newTx, err := txs[1].WithSignature(env.Alice.L2.Signer(), make([]byte, 65))
		txs[1] = newTx
		require.NoError(t, err)
		return block
	})
	env.Batcher.ActL2ChannelClose(t)
	env.Batcher.ActL2BatchSubmit(t)

	// Include the batcher transaction.
	env.Miner.ActL1StartBlock(12)(t)
	env.Miner.ActL1IncludeTxByHash(env.Batcher.LastSubmitted.Hash())(t)
	env.Miner.ActL1EndBlock(t)
	env.Miner.ActL1SafeNext(t)

	// Instruct the sequencer to derive the L2 chain from the data on L1 that the batcher just posted.
	env.Sequencer.ActL1HeadSignal(t)
	env.Sequencer.ActL2PipelineFull(t)

	// Ensure the safe head has not advanced - the batch is invalid.
	l2SafeHead := env.Engine.L2Chain().CurrentSafeBlock()
	require.Equal(t, uint64(0), l2SafeHead.Number.Uint64())

	// Reset the batcher and submit a valid batch.
	env.Batcher.Reset()
	env.Batcher.ActSubmitAll(t)
	env.Miner.ActL1StartBlock(12)(t)
	env.Miner.ActL1IncludeTxByHash(env.Batcher.LastSubmitted.Hash())(t)
	env.Miner.ActL1EndBlock(t)
	env.Miner.ActL1SafeNext(t)

	// Instruct the sequencer to derive the L2 chain from the data on L1 that the batcher just posted.
	env.Sequencer.ActL1HeadSignal(t)
	env.Sequencer.ActL2PipelineFull(t)

	// Ensure the safe head has advanced.
	l1Head := env.Miner.L1Chain().CurrentBlock()
	l2SafeHead = env.Engine.L2Chain().CurrentSafeBlock()
	require.Equal(t, uint64(2), l1Head.Number.Uint64())
	require.Equal(t, uint64(1), l2SafeHead.Number.Uint64())

	env.RunFaultProofProgram(t, l2SafeHead.Number.Uint64(), testCfg.CheckResult, testCfg.InputParams...)
}

func runBadTxInBatch_ResubmitBadFirstFrame_Test(gt *testing.T, testCfg *helpers.TestCfg[any]) {
	t := actionsHelpers.NewDefaultTesting(gt)
	env := helpers.NewL2FaultProofEnv(t, testCfg, helpers.NewTestParams(), helpers.NewBatcherCfg())

	// Build 2 blocks on L2 with 1 tx each.
	for i := 0; i < 2; i++ {
		env.Alice.L2.ActResetTxOpts(t)
		env.Alice.L2.ActSetTxToAddr(&env.Dp.Addresses.Bob)
		env.Alice.L2.ActMakeTx(t)

		env.Sequencer.ActL2StartBlock(t)
		env.Engine.ActL2IncludeTx(env.Alice.Address())(t)
		env.Sequencer.ActL2EndBlock(t)
		env.Alice.L2.ActCheckReceiptStatusOfLastTx(true)(t)
	}

	// Instruct the batcher to submit a faulty channel, with an invalid tx in the second block
	// within the span batch.
	env.Batcher.ActL2BatchBuffer(t)
	err := env.Batcher.Buffer(t, func(block *types.Block) *types.Block {
		// Replace the tx with one that has a bad signature.
		txs := block.Transactions()
		newTx, err := txs[1].WithSignature(env.Alice.L2.Signer(), make([]byte, 65))
		txs[1] = newTx
		require.NoError(t, err)
		return block
	})
	require.NoError(t, err)
	env.Batcher.ActL2ChannelClose(t)
	env.Batcher.ActL2BatchSubmit(t)

	// Include the batcher transaction.
	env.Miner.ActL1StartBlock(12)(t)
	env.Miner.ActL1IncludeTxByHash(env.Batcher.LastSubmitted.Hash())(t)
	env.Miner.ActL1EndBlock(t)
	env.Miner.ActL1SafeNext(t)

	// Instruct the sequencer to derive the L2 chain from the data on L1 that the batcher just posted.
	env.Sequencer.ActL1HeadSignal(t)
	env.Sequencer.ActL2PipelineFull(t)

	// Ensure the safe head has not advanced - the batch is invalid.
	l2SafeHead := env.Engine.L2Chain().CurrentSafeBlock()
	require.Equal(t, uint64(0), l2SafeHead.Number.Uint64())

	// Reset the batcher and submit a valid batch.
	env.Batcher.Reset()
	env.Batcher.ActSubmitAll(t)
	env.Miner.ActL1StartBlock(12)(t)
	env.Miner.ActL1IncludeTxByHash(env.Batcher.LastSubmitted.Hash())(t)
	env.Miner.ActL1EndBlock(t)
	env.Miner.ActL1SafeNext(t)

	// Instruct the sequencer to derive the L2 chain from the data on L1 that the batcher just posted.
	env.Sequencer.ActL1HeadSignal(t)
	env.Sequencer.ActL2PipelineFull(t)

	// Ensure the safe head has advanced.
	l1Head := env.Miner.L1Chain().CurrentBlock()
	l2SafeHead = env.Engine.L2Chain().CurrentSafeBlock()
	require.Equal(t, uint64(2), l1Head.Number.Uint64())
	require.Equal(t, uint64(2), l2SafeHead.Number.Uint64())

	env.RunFaultProofProgram(t, l2SafeHead.Number.Uint64()-1, testCfg.CheckResult, testCfg.InputParams...)
}

func Test_ProgramAction_BadTxInBatch(gt *testing.T) {
	matrix := helpers.NewMatrix[any]()
	defer matrix.Run(gt)

	matrix.AddTestCase(
		"HonestClaim",
		nil,
		helpers.NewForkMatrix(helpers.Granite),
		runBadTxInBatchTest,
		helpers.ExpectNoError(),
	)
	matrix.AddTestCase(
		"JunkClaim",
		nil,
		helpers.NewForkMatrix(helpers.Granite),
		runBadTxInBatchTest,
		helpers.ExpectError(claim.ErrClaimNotValid),
		helpers.WithL2Claim(common.HexToHash("0xdeadbeef")),
	)
	matrix.AddTestCase(
		"ResubmitBadFirstFrame-HonestClaim",
		nil,
		helpers.NewForkMatrix(helpers.Granite),
		runBadTxInBatch_ResubmitBadFirstFrame_Test,
		helpers.ExpectNoError(),
	)
	matrix.AddTestCase(
		"ResubmitBadFirstFrame-JunkClaim",
		nil,
		helpers.NewForkMatrix(helpers.Granite),
		runBadTxInBatch_ResubmitBadFirstFrame_Test,
		helpers.ExpectError(claim.ErrClaimNotValid),
		helpers.WithL2Claim(common.HexToHash("0xdeadbeef")),
	)
}
