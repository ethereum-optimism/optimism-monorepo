package proofs

import (
	"fmt"
	"math/big"
	"testing"

	actionsHelpers "github.com/ethereum-optimism/optimism/op-e2e/actions/helpers"
	"github.com/ethereum-optimism/optimism/op-e2e/actions/proofs/helpers"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils"
	"github.com/ethereum-optimism/optimism/op-program/client/claim"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
)

func Test_ProgramAction_HoloceneInvalidBatch(gt *testing.T) {
	type testCase struct {
		name                    string
		blocks                  []uint // An ordered list of blocks (by number) to add to a single channel.
		useSpanBatch            bool
		blockModifiers          []actionsHelpers.BlockModifier
		breachMaxSequencerDrift bool
		overAdvanceL1Origin     int // block number at which to over-advance
		holoceneExpectations
	}

	// invalidPayload invalidates the signature for the second transaction in the block.
	// This should result in an invalid payload in the engine queue.
	invalidPayload := func(block *types.Block) *types.Block {
		alice := types.NewCancunSigner(big.NewInt(901))
		txs := block.Transactions()
		newTx, err := txs[1].WithSignature(alice, make([]byte, 65))
		if err != nil {
			panic(err)
		}
		txs[1] = newTx
		return block
	}

	// invalidParentHash invalidates the parentHash of the block.
	// This should result in an invalid batch being derived,
	// but only for singular (not for span) batches.
	invalidParentHash := func(block *types.Block) *types.Block {
		headerCopy := block.Header()
		headerCopy.ParentHash = common.MaxHash
		return block.WithSeal(headerCopy)
	}

	k := 2000
	twoThousandBlocks := make([]uint, k)
	for i := 0; i < k; i++ {
		twoThousandBlocks[i] = uint(i) + 1
	}

	// Depending on the blocks list, whether the channel is built as
	// as span batch channel, and whether the blocks are modified / invalidated
	// we expect a different progression of the safe head under Holocene
	// derivation rules, compared with pre Holocene.
	testCases := []testCase{
		// Standard frame submission, standard channel composition
		{
			name: "valid", blocks: []uint{1, 2, 3},
			holoceneExpectations: holoceneExpectations{
				preHolocene: expectations{safeHead: 3}, holocene: expectations{safeHead: 3},
			},
		},

		{
			name: "invalid-payload", blocks: []uint{1, 2, 3}, blockModifiers: []actionsHelpers.BlockModifier{nil, invalidPayload, nil},
			useSpanBatch: false,
			holoceneExpectations: holoceneExpectations{
				preHolocene: expectations{safeHead: 1, // Invalid signature in block 2 causes an invalid _payload_ in the engine queue. Entire span batch is invalidated.
					logs: sequencerOnce("could not process payload attributes"),
				},
				holocene: expectations{safeHead: 2, // We expect the safe head to move to 2 due to creation of a deposit-only block.
					logs: append(
						sequencerOnce("Holocene active, requesting deposits-only attributes"),
						sequencerOnce("could not process payload attributes")...,
					),
				},
			},
		},
		{
			name: "invalid-payload-span", blocks: []uint{1, 2, 3}, blockModifiers: []actionsHelpers.BlockModifier{nil, invalidPayload, nil},
			useSpanBatch: true,
			holoceneExpectations: holoceneExpectations{
				preHolocene: expectations{safeHead: 0, // Invalid signature in block 2 causes an invalid _payload_ in the engine queue. Entire span batch is invalidated.
					logs: sequencerOnce("could not process payload attributes"),
				},

				holocene: expectations{safeHead: 2, // We expect the safe head to move to 2 due to creation of an deposit-only block.
					logs: sequencerOnce("could not process payload attributes"),
				},
			},
		},

		{
			name: "invalid-parent-hash", blocks: []uint{1, 2, 3}, blockModifiers: []actionsHelpers.BlockModifier{nil, invalidParentHash, nil},
			holoceneExpectations: holoceneExpectations{
				preHolocene: expectations{safeHead: 1, // Invalid parentHash in block 2 causes an invalid batch to be dropped.
					logs: sequencerOnce("ignoring batch with mismatching parent hash")},
				holocene: expectations{safeHead: 1, // Same with Holocene.
					logs: sequencerOnce("Dropping invalid singular batch, flushing channel")},
			},
		},
		{
			name: "seq-drift-span", blocks: twoThousandBlocks, // if we artificially stall the l1 origin, this should be enough to trigger violation of the max sequencer drift
			useSpanBatch:            true,
			breachMaxSequencerDrift: true,
			holoceneExpectations: holoceneExpectations{
				preHolocene: expectations{safeHead: 0, // Entire span batch invalidated.
					logs: sequencerOnce("batch exceeded sequencer time drift without adopting next origin, and next L1 origin would have been valid"),
				},
				holocene: expectations{safeHead: 1800, // We expect partial validity until we hit sequencer drift.
					logs: sequencerOnce("batch exceeded sequencer time drift without adopting next origin, and next L1 origin would have been valid"),
				},
			},
		},
		{
			name:                "future-l1-origin-span",
			blocks:              []uint{1, 2, 3, 4},
			useSpanBatch:        true,
			overAdvanceL1Origin: 3, // this will over-advance the L1 origin of block 3
			holoceneExpectations: holoceneExpectations{
				preHolocene: expectations{safeHead: 0, // Entire span batch invalidated.
					logs: sequencerOnce("block timestamp is less than L1 origin timestamp"),
				},
				holocene: expectations{safeHead: 2, // We expect partial validity, safe head should move to block 2, dropping invalid block 3 and remaining channel.
					logs: sequencerOnce("batch timestamp is less than L1 origin timestamp"),
				},
			},
		},
	}

	runHoloceneDerivationTest := func(gt *testing.T, testCfg *helpers.TestCfg[testCase]) {
		t := actionsHelpers.NewDefaultTesting(gt)
		tp := helpers.NewTestParams(func(tp *e2eutils.TestParams) {
			// Set the channel timeout to 10 blocks, 12x lower than the sequencing window.
			tp.ChannelTimeout = 10
		})

		env := helpers.NewL2FaultProofEnv(t, testCfg, tp, helpers.NewBatcherCfg())

		includeBatchTx := func() {
			// Include the last transaction submitted by the batcher.
			env.Miner.ActL1StartBlock(12)(t)
			env.Miner.ActL1IncludeTxByHash(env.Batcher.LastSubmitted.Hash())(t)
			env.Miner.ActL1EndBlock(t)
			// Finalize the block with the first channel frame on L1.
			env.Miner.ActL1SafeNext(t)
			env.Miner.ActL1FinalizeNext(t)
		}

		env.Batcher.ActCreateChannel(t, testCfg.Custom.useSpanBatch)

		max := func(input []uint) uint {
			max := uint(0)
			for _, val := range input {
				if val > max {
					max = val
				}
			}
			return max
		}

		if testCfg.Custom.overAdvanceL1Origin > 0 {
			// Generate future L1 origin or we cannot advance to it.
			env.Miner.ActEmptyBlock(t)
		}

		targetHeadNumber := max(testCfg.Custom.blocks)
		for env.Engine.L2Chain().CurrentBlock().Number.Uint64() < uint64(targetHeadNumber) {
			parentNum := env.Engine.L2Chain().CurrentBlock().Number.Uint64()

			if testCfg.Custom.breachMaxSequencerDrift {
				// prevent L1 origin from progressing
				env.Sequencer.ActL2KeepL1Origin(t)
			} else if oa := testCfg.Custom.overAdvanceL1Origin; oa > 0 && oa == int(parentNum)+1 {
				env.Sequencer.ActL2ForceAdvanceL1Origin(t)
			}

			env.Sequencer.ActL2StartBlock(t)

			if !testCfg.Custom.breachMaxSequencerDrift {
				// Send an L2 tx
				env.Alice.L2.ActResetTxOpts(t)
				env.Alice.L2.ActSetTxToAddr(&env.Dp.Addresses.Bob)
				env.Alice.L2.ActMakeTx(t)
				env.Engine.ActL2IncludeTx(env.Alice.Address())(t)
			}

			if testCfg.Custom.breachMaxSequencerDrift &&
				parentNum == 1799 ||
				parentNum == 1800 ||
				parentNum == 1801 {
				// Send an L2 tx and force sequencer to include it
				env.Alice.L2.ActResetTxOpts(t)
				env.Alice.L2.ActSetTxToAddr(&env.Dp.Addresses.Bob)
				env.Alice.L2.ActMakeTx(t)
				env.Engine.ActL2IncludeTxIgnoreForcedEmpty(env.Alice.Address())(t)
			}

			env.Sequencer.ActL2EndBlock(t)
		}

		// Buffer the blocks in the batcher.
		for i, blockNum := range testCfg.Custom.blocks {
			var blockModifier actionsHelpers.BlockModifier
			if len(testCfg.Custom.blockModifiers) > i {
				blockModifier = testCfg.Custom.blockModifiers[i]
			}
			env.Batcher.ActAddBlockByNumber(t, int64(blockNum), blockModifier, actionsHelpers.BlockLogger(t))

		}

		env.Batcher.ActL2ChannelClose(t)
		frame := env.Batcher.ReadNextOutputFrame(t)
		require.NotEmpty(t, frame)
		env.Batcher.ActL2BatchSubmitRaw(t, frame)
		includeBatchTx()

		// Instruct the sequencer to derive the L2 chain from the data on L1 that the batcher just posted.
		env.Sequencer.ActL1HeadSignal(t)
		env.Sequencer.ActL2PipelineFull(t)

		l2SafeHead := env.Sequencer.L2Safe()

		isHolocene := testCfg.Hardfork.Precedence >= helpers.Holocene.Precedence
		testCfg.Custom.RequireExpectedProgressAndLogs(t, l2SafeHead, isHolocene, env.Engine, env.Logs)
		t.Log("Safe head progressed as expected", "l2SafeHeadNumber", l2SafeHead.Number)

		if safeHeadNumber := l2SafeHead.Number; safeHeadNumber > 0 {
			env.RunFaultProofProgram(t, safeHeadNumber, testCfg.CheckResult, testCfg.InputParams...)
		}
	}

	matrix := helpers.NewMatrix[testCase]()
	defer matrix.Run(gt)

	for _, ordering := range testCases {
		matrix.AddTestCase(
			fmt.Sprintf("HonestClaim-%s", ordering.name),
			ordering,
			helpers.NewForkMatrix(helpers.Granite, helpers.LatestFork),
			runHoloceneDerivationTest,
			helpers.ExpectNoError(),
		)
		matrix.AddTestCase(
			fmt.Sprintf("JunkClaim-%s", ordering.name),
			ordering,
			helpers.NewForkMatrix(helpers.Granite, helpers.LatestFork),
			runHoloceneDerivationTest,
			helpers.ExpectError(claim.ErrClaimNotValid),
			helpers.WithL2Claim(common.HexToHash("0xdeadbeef")),
		)
	}
}
