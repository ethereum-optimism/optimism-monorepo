package proofs

import (
	"testing"

	"github.com/ethereum-optimism/optimism/op-chain-ops/genesis"
	actionsHelpers "github.com/ethereum-optimism/optimism/op-e2e/actions/helpers"
	"github.com/ethereum-optimism/optimism/op-e2e/actions/proofs/helpers"
	"github.com/ethereum-optimism/optimism/op-program/client/claim"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/stretchr/testify/require"
)

func Test_ProgramAction_HoloceneActivation(gt *testing.T) {

	runHoloceneDerivationTest := func(gt *testing.T, testCfg *helpers.TestCfg[any]) {
		t := actionsHelpers.NewDefaultTesting(gt)

		// Define override to activate Holocene 14 seconds after genesis
		var setHoloceneTime = func(dc *genesis.DeployConfig) {
			fourteen := hexutil.Uint64(14)
			dc.L2GenesisHoloceneTimeOffset = &fourteen
		}

		env := helpers.NewL2FaultProofEnv(t, testCfg, helpers.NewTestParams(), helpers.NewBatcherCfg(), setHoloceneTime)

		t.Log("HoloceneTime:  ", env.Sequencer.RollupCfg.HoloceneTime)

		// Build the L2 chain
		blocks := []uint{1, 2}
		targetHeadNumber := 2
		for env.Engine.L2Chain().CurrentBlock().Number.Uint64() < uint64(targetHeadNumber) {
			env.Sequencer.ActL2StartBlock(t)
			// Send an L2 tx
			env.Alice.L2.ActResetTxOpts(t)
			env.Alice.L2.ActSetTxToAddr(&env.Dp.Addresses.Bob)
			env.Alice.L2.ActMakeTx(t)
			env.Engine.ActL2IncludeTx(env.Alice.Address())(t)
			env.Sequencer.ActL2EndBlock(t)
		}

		// Build up a local list of frames
		orderedFrames := make([][]byte, 0, 2)
		// Buffer the blocks in the batcher and populate orderedFrames list
		env.Batcher.ActCreateChannel(t, false)
		for i, blockNum := range blocks {
			env.Batcher.ActAddBlockByNumber(t, int64(blockNum), actionsHelpers.BlockLogger(t))
			if i == len(blocks)-1 {
				env.Batcher.ActL2ChannelClose(t)
			}
			frame := env.Batcher.ReadNextOutputFrame(t)
			require.NotEmpty(t, frame, "frame %d", i)
			orderedFrames = append(orderedFrames, frame)
		}

		includeBatchTx := func() {
			// Include the last transaction submitted by the batcher.
			env.Miner.ActL1StartBlock(12)(t)
			env.Miner.ActL1IncludeTxByHash(env.Batcher.LastSubmitted.Hash())(t)
			env.Miner.ActL1EndBlock(t)
		}

		// Submit first frame
		env.Batcher.ActL2BatchSubmitRaw(t, orderedFrames[0])
		includeBatchTx() // block should have a timestamp of 12s after genesis

		// Holocene should activate 14s after genesis, so that the previous l1 block
		// was before HoloceneTime and the next l1 block is after it

		// Submit final frame
		env.Batcher.ActL2BatchSubmitRaw(t, orderedFrames[1])
		includeBatchTx() // block should have a timestamp of 24s after genesis

		// Instruct the sequencer to derive the L2 chain from the data on L1 that the batcher just posted.
		env.Sequencer.ActL1HeadSignal(t)
		env.Sequencer.ActL2PipelineFull(t)

		l2SafeHead := env.Sequencer.L2Safe()
		require.EqualValues(t, uint64(0), l2SafeHead.Number) // channel should be dropped, so no safe head progression
		t.Log("Safe head progressed as expected", "l2SafeHeadNumber", l2SafeHead.Number)

		// Log assertions
		filters := []string{
			"FrameQueue: resetting with Holocene activation",
			"ChannelMux: transforming to Holocene stage",
			"BatchMux: transforming to Holocene stage",
			"dropping non-first frame without channel",
		}
		for _, filter := range filters {
			recs := env.Logs.FindLogs(testlog.NewMessageContainsFilter(filter), testlog.NewAttributesFilter("role", "sequencer"))
			require.Len(t, recs, 1, "searching for %d instances of '%s' in logs from role %s", 1, filter, "sequencer")
		}

		env.RunFaultProofProgram(t, l2SafeHead.Number, testCfg.CheckResult, testCfg.InputParams...)
	}

	matrix := helpers.NewMatrix[any]()
	defer matrix.Run(gt)

	matrix.AddTestCase(
		"HonestClaim-HoloceneActivation",
		nil,
		helpers.NewForkMatrix(helpers.Granite),
		runHoloceneDerivationTest,
		helpers.ExpectNoError(),
	)
	matrix.AddTestCase(
		"JunkClaim-HoloceneActivation",
		nil,
		helpers.NewForkMatrix(helpers.Granite),
		runHoloceneDerivationTest,
		helpers.ExpectError(claim.ErrClaimNotValid),
		helpers.WithL2Claim(common.HexToHash("0xdeadbeef")),
	)
}
