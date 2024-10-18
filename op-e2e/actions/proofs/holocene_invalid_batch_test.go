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
		isSpanBatch             bool
		blockModifiers          []actionsHelpers.BlockModifier
		breachMaxSequencerDrift bool
		overAdvanceL1Origin     bool
		holoceneExpectations
	}

	// invalidPayload invalidates the signature for the second transaction in the block.
	// This should result in an invalid payload in the engine queue.
	var invalidPayload = func(block *types.Block) *types.Block {
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
	var invalidParentHash = func(block *types.Block) *types.Block {
		headerCopy := block.Header()
		headerCopy.ParentHash = common.MaxHash
		return block.WithSeal(headerCopy)
	}

	k := 2000
	var twoThousandBlocks = make([]uint, k)
	for i := 0; i < k; i++ {
		twoThousandBlocks[i] = uint(i) + 1
	}

	partiallyValidSpanBatchFrame := []byte{
		0,                                                                           // version_byte
		207, 193, 36, 120, 131, 193, 183, 227, 45, 196, 92, 103, 218, 173, 173, 192, // channel_id
		0, 0, // frame_number
		0, 0, 2, 58, // frame_data_length = 570 bytes
		// BEGIN frame_data
		120, 1, // zlib header
		0, 42, 2, 213, 253, // initial DEFLATE block indicating 554 uncompressed bytes
		185, 2, 39, // rlp prefix for long string 569 bytes
		//// BEGIN encoded span batch
		1,                                                                                          // batch_version (span)
		1,                                                                                          // rel_timestamp
		1,                                                                                          // l1_origin_num
		137, 125, 54, 181, 103, 34, 6, 165, 204, 60, 141, 71, 165, 172, 31, 148, 75, 246, 120, 219, // parent_check
		79, 189, 89, 43, 191, 92, 73, 34, 21, 146, 178, 246, 188, 199, 119, 72, 77, 120, 66, 191, // l1_origin_check
		6,                // block_count
		4,                // origin_bits 0b000100 -- here's where we make the 4th block invalid, by advancing the l1_origin (+12s) so it is ahead of the block timestamp (+8s)
		1, 1, 1, 1, 1, 1, // block_tx_counts
		// txs (not broken down here):
		63, 2, 8, 224, 4, 89, 43, 169, 191, 250, 124, 3, 145, 72, 46, 23, 29, 16, 64, 186, 32, 226, 85, 58, 152, 158, 64, 16, 147, 44, 134, 199, 179, 5, 66, 34, 121, 141, 214, 128, 187, 62, 59, 172, 109, 72, 7, 96, 10, 91, 221, 190, 58, 214, 243, 19, 175, 160, 252, 152, 216, 203, 106, 120, 54, 122, 85, 66, 157, 73, 172, 176, 195, 53, 71, 163, 114, 211, 248, 81, 77, 58, 69, 14, 116, 157, 33, 160, 242, 210, 117, 137, 203, 26, 115, 181, 24, 243, 113, 185, 45, 230, 246, 26, 148, 19, 18, 181, 9, 67, 240, 253, 156, 52, 142, 188, 255, 136, 176, 146, 9, 115, 233, 79, 128, 239, 98, 105, 145, 13, 214, 245, 165, 70, 93, 34, 53, 201, 58, 86, 81, 153, 245, 214, 222, 235, 35, 125, 248, 119, 136, 226, 99, 38, 217, 238, 213, 227, 209, 77, 68, 12, 120, 171, 98, 56, 100, 72, 135, 11, 223, 87, 230, 79, 74, 94, 80, 244, 41, 10, 82, 52, 254, 58, 158, 184, 13, 16, 199, 54, 229, 101, 227, 182, 7, 88, 96, 154, 168, 39, 92, 201, 197, 241, 5, 174, 52, 209, 34, 15, 241, 73, 92, 123, 55, 247, 55, 33, 48, 170, 68, 20, 174, 90, 169, 95, 6, 36, 105, 122, 14, 143, 66, 133, 102, 248, 144, 226, 246, 10, 177, 152, 135, 67, 65, 58, 183, 90, 181, 125, 180, 254, 90, 154, 152, 87, 243, 249, 195, 28, 21, 47, 229, 13, 226, 162, 135, 186, 172, 31, 143, 207, 66, 14, 63, 43, 138, 215, 237, 40, 155, 71, 13, 37, 194, 72, 196, 144, 174, 1, 1, 214, 40, 144, 44, 173, 71, 17, 168, 31, 106, 78, 198, 244, 240, 223, 88, 166, 215, 200, 52, 148, 209, 124, 128, 197, 164, 204, 161, 185, 18, 170, 180, 9, 82, 214, 186, 63, 244, 213, 132, 147, 12, 118, 37, 79, 145, 197, 4, 80, 10, 61, 190, 48, 154, 196, 44, 176, 46, 228, 99, 144, 237, 208, 39, 112, 1, 110, 111, 135, 38, 206, 24, 208, 218, 234, 94, 171, 109, 151, 34, 74, 241, 89, 83, 190, 194, 140, 52, 1, 194, 80, 80, 71, 30, 254, 2, 205, 128, 132, 119, 53, 148, 0, 132, 119, 53, 148, 2, 128, 192, 2, 205, 128, 132, 119, 53, 148, 0, 132, 119, 53, 148, 2, 128, 192, 2, 205, 128, 132, 119, 53, 148, 0, 132, 119, 53, 148, 2, 128, 192, 2, 205, 128, 132, 119, 53, 148, 0, 132, 119, 53, 148, 2, 128, 192, 2, 205, 128, 132, 119, 53, 148, 0, 132, 119, 53, 148, 2, 128, 192, 2, 205, 128, 132, 119, 53, 148, 0, 132, 119, 53, 148, 2, 128, 192, 0, 1, 2, 3, 4, 5, 161, 164, 3, 161, 164, 3, 161, 164, 3, 161, 164, 3, 161, 164, 3, 161, 164, 3,
		//// END encoded span batch
		1, 0, 0, 255, 255, // terminal DEFLATE block
		199, 158, 250, 29, // 4-byte Adler-32 checksum
		// END frame_data
		1, // is_last
	}

	// Depending on the blocks list, whether the channel is built as
	// as span batch channel, and whether the blocks are modified / invalidated
	// we expect a different progression of the safe head under Holocene
	// derivation rules, compared with pre Holocene.
	var testCases = []testCase{
		// Standard frame submission, standard channel composition
		{name: "case-0", blocks: []uint{1, 2, 3},
			holoceneExpectations: holoceneExpectations{
				safeHeadPreHolocene: 3, safeHeadHolocene: 3,
			},
		},

		{name: "case-3a", blocks: []uint{1, 2, 3}, blockModifiers: []actionsHelpers.BlockModifier{nil, invalidPayload, nil},
			isSpanBatch: true,
			holoceneExpectations: holoceneExpectations{
				safeHeadPreHolocene: 0, // Invalid signature in block 2 causes an invalid _payload_ in the engine queue. Entire span batch is invalidated.
				safeHeadHolocene:    0, // TODO with full Holocene implementation, we expect the safe head to move to 2 due to creation of an deposit-only block.
			},
		},
		{name: "case-3b", blocks: []uint{1, 2, 3}, blockModifiers: []actionsHelpers.BlockModifier{nil, invalidParentHash, nil},
			holoceneExpectations: holoceneExpectations{
				safeHeadPreHolocene: 1, // Invalid parentHash in block 2 causes an invalid batch to be derived.
				safeHeadHolocene:    1, // Invalid parentHash in block 2 causes an invalid batch to be derived. This batch + remaining channel is dropped.
			},
		},
		{name: "case-3c", blocks: twoThousandBlocks, // if we artificially stall the l1 origin, this should be enough to trigger violation of the max sequencer drift
			isSpanBatch:             true,
			breachMaxSequencerDrift: true,
			holoceneExpectations: holoceneExpectations{
				safeHeadPreHolocene: 0, // entire span batch invalidated
				safeHeadHolocene:    0, // TODO we expect partial validity, safe head should move to  block 1800. So far only pending safe head moves.
			},
		},
		{name: "case-3d",
			isSpanBatch:         true,
			overAdvanceL1Origin: true, // this will trigger the use of the partiallyValidSpanBatchFrame and bypass the sequencer entirely
			holoceneExpectations: holoceneExpectations{
				safeHeadPreHolocene: 0, // entire span batch invalidated
				safeHeadHolocene:    0, // TODO we expect partial validity, safe head should move to block 4.  So far only pending safe head moves.
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

		env.Batcher.ActCreateChannel(t, testCfg.Custom.isSpanBatch)

		var max = func(input []uint) uint {
			max := uint(0)
			for _, val := range input {
				if val > max {
					max = val
				}
			}
			return max
		}

		targetHeadNumber := max(testCfg.Custom.blocks)
		for env.Engine.L2Chain().CurrentBlock().Number.Uint64() < uint64(targetHeadNumber) {

			if testCfg.Custom.breachMaxSequencerDrift {
				// prevent L1 origin from progressing
				env.Sequencer.ActL2KeepL1Origin(t)
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
				env.Engine.L2Chain().CurrentBlock().Number.Uint64() == 1799 ||
				env.Engine.L2Chain().CurrentBlock().Number.Uint64() == 1800 ||
				env.Engine.L2Chain().CurrentBlock().Number.Uint64() == 1801 {
				// Send an L2 tx and force sequencer to include it
				env.Alice.L2.ActResetTxOpts(t)
				env.Alice.L2.ActSetTxToAddr(&env.Dp.Addresses.Bob)
				env.Alice.L2.ActMakeTx(t)
				env.Engine.ActL2IncludeTxIgnoreForcedEmpty(env.Alice.Address())(t)
			}

			env.Sequencer.ActL2EndBlock(t)
		}

		if testCfg.Custom.overAdvanceL1Origin {
			// Here we bypass the sequencer entirely, and simply submit a hand rolled batcher tx
			env.Batcher.ActL2BatchSubmitRaw(t, partiallyValidSpanBatchFrame)
			includeBatchTx()
		} else {
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
		}

		// Instruct the sequencer to derive the L2 chain from the data on L1 that the batcher just posted.
		env.Sequencer.ActL1HeadSignal(t)
		env.Sequencer.ActL2PipelineFull(t)

		l2SafeHead := env.Sequencer.L2Safe()

		testCfg.Custom.RequireExpectedProgress(t, l2SafeHead, testCfg.Hardfork.Precedence < helpers.Holocene.Precedence, env.Engine)

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
