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

func runL1LookbackTest(gt *testing.T, testCfg *helpers.TestCfg[any]) {
	t := actionsHelpers.NewDefaultTesting(gt)
	tp := helpers.NewTestParams()
	env := helpers.NewL2FaultProofEnv(t, testCfg, tp, helpers.NewBatcherCfg())

	const numL2Blocks = 8
	for i := 0; i < numL2Blocks; i++ {
		// Create an empty L2 block.
		env.Sequencer.ActL2StartBlock(t)
		env.Sequencer.ActL2EndBlock(t)

		// Buffer the L2 block in the batcher.
		env.Batcher.ActBufferAll(t)
		if i == numL2Blocks-1 {
			env.Batcher.ActL2ChannelClose(t)
		}
		env.Batcher.ActL2BatchSubmit(t)

		// Include the frame on L1.
		env.Miner.ActL1StartBlock(12)(t)
		env.Miner.ActL1IncludeTxByHash(env.Batcher.LastSubmitted.Hash())(t)
		env.Miner.ActL1EndBlock(t)
		env.Miner.ActL1SafeNext(t)
	}

	// Instruct the sequencer to derive the L2 chain from the data on L1 that the batcher just posted.
	env.Sequencer.ActL1HeadSignal(t)
	env.Sequencer.ActL2PipelineFull(t)

	// Ensure that the safe head has advanced to `NumL2Blocks`.
	l2SafeHead := env.Engine.L2Chain().CurrentSafeBlock()
	require.EqualValues(t, numL2Blocks, l2SafeHead.Number.Uint64())

	// Run the FPP on the configured L2 block.
	env.RunFaultProofProgram(t, numL2Blocks/2, testCfg.CheckResult, testCfg.InputParams...)
}

func runL1LookbackTest_ReopenChannel(gt *testing.T, testCfg *helpers.TestCfg[any]) {
	t := actionsHelpers.NewDefaultTesting(gt)
	tp := helpers.NewTestParams()
	env := helpers.NewL2FaultProofEnv(t, testCfg, tp, helpers.NewBatcherCfg())

	// Create an L2 block with 1 transaction.
	env.Sequencer.ActL2StartBlock(t)
	env.Alice.L2.ActResetTxOpts(t)
	env.Alice.L2.ActSetTxToAddr(&env.Dp.Addresses.Bob)
	env.Alice.L2.ActMakeTx(t)
	env.Engine.ActL2IncludeTx(env.Alice.Address())(t)
	env.Sequencer.ActL2EndBlock(t)
	l2BlockBeforeDerive := env.Engine.L2Chain().CurrentBlock()

	// Buffer the L2 block in the batcher.
	env.Batcher.ActL2BatchBuffer(t)
	env.Batcher.ActL2BatchSubmit(t)

	// Include the frame on L1.
	env.Miner.ActL1StartBlock(12)(t)
	env.Miner.ActL1IncludeTxByHash(env.Batcher.LastSubmitted.Hash())(t)
	env.Miner.ActL1EndBlock(t)
	env.Miner.ActL1SafeNext(t)

	// Re-submit the first L2 block frame w/ different transaction data.
	err := env.Batcher.Buffer(t, func(block *types.Block) *types.Block {
		env.Bob.L2.ActResetTxOpts(t)
		env.Bob.L2.ActSetTxToAddr(&env.Dp.Addresses.Mallory)
		tx := env.Bob.L2.MakeTransaction(t)
		block.Transactions()[1] = tx
		return block
	})
	require.NoError(t, err)
	env.Batcher.ActL2BatchSubmit(t)

	// Include the duplicate frame on L1.
	env.Miner.ActL1StartBlock(12)(t)
	env.Miner.ActL1IncludeTxByHash(env.Batcher.LastSubmitted.Hash())(t)
	env.Miner.ActL1EndBlock(t)
	env.Miner.ActL1SafeNext(t)

	const numL2Blocks = 8
	for i := 1; i < numL2Blocks; i++ {
		// Create an empty L2 block.
		env.Sequencer.ActL2StartBlock(t)
		env.Sequencer.ActL2EndBlock(t)

		// Buffer the L2 block in the batcher.
		env.Batcher.ActBufferAll(t)
		if i == numL2Blocks-1 {
			env.Batcher.ActL2ChannelClose(t)
		}
		env.Batcher.ActL2BatchSubmit(t)

		// Include the frame on L1.
		env.Miner.ActL1StartBlock(12)(t)
		env.Miner.ActL1IncludeTxByHash(env.Batcher.LastSubmitted.Hash())(t)
		env.Miner.ActL1EndBlock(t)
		env.Miner.ActL1SafeNext(t)
	}

	// Instruct the sequencer to derive the L2 chain from the data on L1 that the batcher just posted.
	env.Sequencer.ActL1HeadSignal(t)
	env.Sequencer.ActL2PipelineFull(t)

	// Ensure that the correct block was derived.
	l2BlockAfterDerive := env.Engine.L2Chain().GetBlockByNumber(1)
	require.EqualValues(t, l2BlockAfterDerive.Hash(), l2BlockBeforeDerive.Hash())

	// Ensure that the safe head has advanced to `NumL2Blocks`.
	l2SafeHead := env.Engine.L2Chain().CurrentSafeBlock()
	require.EqualValues(t, numL2Blocks, l2SafeHead.Number.Uint64())

	// Run the FPP on the configured L2 block.
	env.RunFaultProofProgram(t, numL2Blocks/2, testCfg.CheckResult, testCfg.InputParams...)
}

func Test_ProgramAction_L1Lookback(gt *testing.T) {
	matrix := helpers.NewMatrix[any]()
	defer matrix.Run(gt)

	matrix.AddTestCase(
		"HonestClaim",
		nil,
		helpers.LatestForkOnly,
		runL1LookbackTest,
		helpers.ExpectNoError(),
	)
	matrix.AddTestCase(
		"JunkClaim",
		nil,
		helpers.LatestForkOnly,
		runL1LookbackTest,
		helpers.ExpectError(claim.ErrClaimNotValid),
		helpers.WithL2Claim(common.HexToHash("0xdeadbeef")),
	)
	matrix.AddTestCase(
		"HonestClaim-ReopenChannel",
		nil,
		helpers.LatestForkOnly,
		runL1LookbackTest_ReopenChannel,
		helpers.ExpectNoError(),
	)
	matrix.AddTestCase(
		"JunkClaim-ReopenChannel",
		nil,
		helpers.LatestForkOnly,
		runL1LookbackTest_ReopenChannel,
		helpers.ExpectError(claim.ErrClaimNotValid),
		helpers.WithL2Claim(common.HexToHash("0xdeadbeef")),
	)
}
