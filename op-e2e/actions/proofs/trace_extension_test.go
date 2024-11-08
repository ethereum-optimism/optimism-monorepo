package proofs

import (
	"testing"

	actionsHelpers "github.com/ethereum-optimism/optimism/op-e2e/actions/helpers"
	"github.com/ethereum-optimism/optimism/op-e2e/actions/proofs/helpers"
	"github.com/ethereum-optimism/optimism/op-program/client/claim"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func runSafeHeadTraceExtensionTest(gt *testing.T, testCfg *helpers.TestCfg[any]) {
	t := actionsHelpers.NewDefaultTesting(gt)
	env := helpers.NewL2FaultProofEnv(t, testCfg, helpers.NewTestParams(), helpers.NewBatcherCfg())

	// Build an empty block on L2
	env.Sequencer.ActL2StartBlock(t)
	env.Sequencer.ActL2EndBlock(t)

	// Instruct the batcher to submit the block to L1, and include the transaction.
	env.Batcher.ActSubmitAll(t)
	env.Miner.ActL1StartBlock(12)(t)
	env.Miner.ActL1IncludeTxByHash(env.Batcher.LastSubmitted.Hash())(t)
	env.Miner.ActL1EndBlock(t)

	// Instruct the sequencer to derive the L2 chain from the data on L1 that the batcher just posted.
	env.Sequencer.ActL1HeadSignal(t)
	env.Sequencer.ActL2PipelineFull(t)

	l1Head := env.Miner.L1Chain().CurrentBlock()
	l2SafeHead := env.Engine.L2Chain().CurrentSafeBlock()

	// Ensure there is only 1 block on L1.
	require.Equal(t, uint64(1), l1Head.Number.Uint64())
	// Ensure the block is marked as safe before we attempt to fault prove it.
	require.Equal(t, uint64(1), l2SafeHead.Number.Uint64())

	// Set claimed L2 block number to be past the actual safe head (still using the safe head output as the claim)
	params := []helpers.FixtureInputParam{helpers.WithL2BlockNumber(l2SafeHead.Number.Uint64() + 1)}
	params = append(params, testCfg.InputParams...)
	env.RunFaultProofProgram(t, l2SafeHead.Number.Uint64(), testCfg.CheckResult, params...)
}

// Test_ProgramAction_SafeHeadTraceExtension checks that op-program correctly handles the trace extension case where
// the claimed l2 block number is after the safe head. The honest actor should repeat the output root from the safe head
// and op-program should consider it valid even though the claimed l2 block number is not reached.
// Output roots other than from the safe head should be invalid if the claimed l2 block number is not reached.
func Test_ProgramAction_SafeHeadTraceExtension(gt *testing.T) {
	matrix := helpers.NewMatrix[any]()
	defer matrix.Run(gt)

	matrix.AddTestCase(
		"HonestClaim",
		nil,
		helpers.LatestForkOnly,
		runSafeHeadTraceExtensionTest,
		helpers.ExpectNoError(),
	)
	matrix.AddTestCase(
		"JunkClaim",
		nil,
		helpers.LatestForkOnly,
		runSafeHeadTraceExtensionTest,
		helpers.ExpectError(claim.ErrClaimNotValid),
		helpers.WithL2Claim(common.HexToHash("0xdeadbeef")),
	)
}
