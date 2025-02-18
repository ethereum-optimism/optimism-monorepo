package batcher

import (
	"math/big"
	"testing"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/queue"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"
)

type testChannelStatuser struct {
	latestL2                 eth.BlockID
	inclusionBlock           uint64
	fullySubmitted, timedOut bool
}

func (tcs testChannelStatuser) LatestL2() eth.BlockID {
	return tcs.latestL2
}

func (tcs testChannelStatuser) MaxInclusionBlock() uint64 {
	return tcs.inclusionBlock
}
func (tcs testChannelStatuser) isFullySubmitted() bool {
	return tcs.fullySubmitted
}

func (tcs testChannelStatuser) isTimedOut() bool {
	return tcs.timedOut
}

func TestBatchSubmitter_computeSyncActions(t *testing.T) {

	block101 := types.NewBlockWithHeader(&types.Header{Number: big.NewInt(101)})
	block102 := types.NewBlockWithHeader(&types.Header{Number: big.NewInt(102)})
	block103 := types.NewBlockWithHeader(&types.Header{Number: big.NewInt(103)})

	channel103 := testChannelStatuser{
		latestL2:       eth.ToBlockID(block103),
		inclusionBlock: 1,
		fullySubmitted: true,
		timedOut:       false,
	}

	block104 := types.NewBlockWithHeader(&types.Header{Number: big.NewInt(104)})

	channel104 := testChannelStatuser{
		latestL2:       eth.ToBlockID(block104),
		inclusionBlock: 1,
		fullySubmitted: false,
		timedOut:       false,
	}

	happyCaseLogs := []string{} // in the happy case we expect no logs

	type TestCase struct {
		name string
		// inputs
		newSyncStatus eth.SyncStatus
		prevCurrentL1 eth.L1BlockRef
		blocks        queue.Queue[*types.Block]
		channels      []channelStatuser
		// expectations
		expected             syncActions
		expectedSeqOutOfSync bool
		expectedLogs         []string
	}

	testCases := []TestCase{
		{name: "empty sync status",
			// This can happen when the sequencer recovers from a reorg
			newSyncStatus:        eth.SyncStatus{},
			expected:             syncActions{},
			expectedSeqOutOfSync: true,
			expectedLogs:         []string{"empty sync status"},
		},
		{name: "current l1 reversed",
			// This can happen when the sequencer restarts or is switched
			// to a backup sequencer:
			newSyncStatus: eth.SyncStatus{
				HeadL1:    eth.BlockRef{Number: 2},
				CurrentL1: eth.BlockRef{Number: 1},
			},
			prevCurrentL1:        eth.BlockRef{Number: 2},
			expected:             syncActions{},
			expectedSeqOutOfSync: true,
			expectedLogs:         []string{"sequencer currentL1 reversed"},
		},
		{name: "gap between safe chain and state",
			// This can happen if there is an L1 reorg:
			// although the sequencer has derived up the same
			// L1 block height, it derived fewer safe L2 blocks.
			newSyncStatus: eth.SyncStatus{
				HeadL1:      eth.BlockRef{Number: 6},
				CurrentL1:   eth.BlockRef{Number: 1},
				LocalSafeL2: eth.L2BlockRef{Number: 100, L1Origin: eth.BlockID{Number: 1}},
				UnsafeL2:    eth.L2BlockRef{Number: 109},
			},
			prevCurrentL1: eth.BlockRef{Number: 1},
			blocks:        queue.Queue[*types.Block]{block102, block103}, // note absence of block101
			channels:      []channelStatuser{channel103},
			expected: syncActions{
				clearState:   &eth.BlockID{Number: 1},
				blocksToLoad: &inclusiveBlockRange{101, 109},
			},
			expectedLogs: []string{"next safe block is below oldest block in state"},
		},
		{name: "unexpectedly good progress",
			// This can happen if another batcher instance got some blocks
			// included in the safe chain:
			newSyncStatus: eth.SyncStatus{
				HeadL1:      eth.BlockRef{Number: 6},
				CurrentL1:   eth.BlockRef{Number: 2},
				LocalSafeL2: eth.L2BlockRef{Number: 104, L1Origin: eth.BlockID{Number: 1}},
				UnsafeL2:    eth.L2BlockRef{Number: 109},
			},
			prevCurrentL1: eth.BlockRef{Number: 1},
			blocks:        queue.Queue[*types.Block]{block101, block102, block103},
			channels:      []channelStatuser{channel103},
			expected: syncActions{
				clearState:   &eth.BlockID{Number: 1},
				blocksToLoad: &inclusiveBlockRange{105, 109},
			},
			expectedLogs: []string{"safe head above newest block in state"},
		},
		{name: "safe chain reorg",
			// This can happen if there is an L1 reorg, the safe chain is at an acceptable
			// height but it does not descend from the blocks in state:
			newSyncStatus: eth.SyncStatus{
				HeadL1:      eth.BlockRef{Number: 5},
				CurrentL1:   eth.BlockRef{Number: 2},
				LocalSafeL2: eth.L2BlockRef{Number: 103, Hash: block101.Hash(), L1Origin: eth.BlockID{Number: 1}}, // note hash mismatch
				UnsafeL2:    eth.L2BlockRef{Number: 109},
			},
			prevCurrentL1: eth.BlockRef{Number: 1},
			blocks:        queue.Queue[*types.Block]{block101, block102, block103},
			channels:      []channelStatuser{channel103},
			expected: syncActions{
				clearState:   &eth.BlockID{Number: 1},
				blocksToLoad: &inclusiveBlockRange{104, 109},
			},
			expectedLogs: []string{"safe chain reorg"},
		},
		{name: "failed to make expected progress",
			// This could happen if the batcher unexpectedly violates the
			// Holocene derivation rules:
			newSyncStatus: eth.SyncStatus{
				HeadL1:      eth.BlockRef{Number: 3},
				CurrentL1:   eth.BlockRef{Number: 2},
				LocalSafeL2: eth.L2BlockRef{Number: 101, Hash: block101.Hash(), L1Origin: eth.BlockID{Number: 1}},
				UnsafeL2:    eth.L2BlockRef{Number: 109},
			},
			prevCurrentL1: eth.BlockRef{Number: 1},
			blocks:        queue.Queue[*types.Block]{block101, block102, block103},
			channels:      []channelStatuser{channel103},
			expected: syncActions{
				clearState:   &eth.BlockID{Number: 1},
				blocksToLoad: &inclusiveBlockRange{102, 109},
			},
			expectedLogs: []string{"sequencer did not make expected progress"},
		},
		{name: "failed to make expected progress (unsafe=safe)",
			// Edge case where unsafe = safe
			newSyncStatus: eth.SyncStatus{
				HeadL1:      eth.BlockRef{Number: 3},
				CurrentL1:   eth.BlockRef{Number: 2},
				LocalSafeL2: eth.L2BlockRef{Number: 101, Hash: block101.Hash(), L1Origin: eth.BlockID{Number: 1}},
				UnsafeL2:    eth.L2BlockRef{Number: 101},
			},
			prevCurrentL1: eth.BlockRef{Number: 1},
			blocks:        queue.Queue[*types.Block]{block102, block103},
			channels:      []channelStatuser{channel103},
			expected: syncActions{
				clearState: &eth.BlockID{Number: 1},
				// no blocks to load since there are no unsafe blocks
			},
			expectedLogs: []string{"sequencer did not make expected progress"},
		},
		{name: "no progress",
			// This can happen if we have a long channel duration
			// and we didn't submit or have any txs confirmed since
			// the last sync.
			newSyncStatus: eth.SyncStatus{
				HeadL1:      eth.BlockRef{Number: 4},
				CurrentL1:   eth.BlockRef{Number: 1},
				LocalSafeL2: eth.L2BlockRef{Number: 100},
				UnsafeL2:    eth.L2BlockRef{Number: 109},
			},
			prevCurrentL1: eth.BlockRef{Number: 1},
			blocks:        queue.Queue[*types.Block]{block101, block102, block103},
			channels:      []channelStatuser{channel103},
			expected: syncActions{
				blocksToLoad: &inclusiveBlockRange{104, 109},
			},
		},
		{name: "no blocks",
			// This happens when the batcher is starting up for the first time
			newSyncStatus: eth.SyncStatus{
				HeadL1:      eth.BlockRef{Number: 5},
				CurrentL1:   eth.BlockRef{Number: 2},
				LocalSafeL2: eth.L2BlockRef{Number: 103, Hash: block103.Hash()},
				UnsafeL2:    eth.L2BlockRef{Number: 109},
			},
			prevCurrentL1: eth.BlockRef{Number: 1},
			blocks:        queue.Queue[*types.Block]{},
			channels:      []channelStatuser{},
			expected: syncActions{
				blocksToLoad: &inclusiveBlockRange{104, 109},
			},
			expectedLogs: []string{"no blocks in state"},
		},
		{name: "happy path",
			// This happens when the safe chain is being progressed as expected:
			newSyncStatus: eth.SyncStatus{
				HeadL1:      eth.BlockRef{Number: 5},
				CurrentL1:   eth.BlockRef{Number: 2},
				LocalSafeL2: eth.L2BlockRef{Number: 103, Hash: block103.Hash()},
				UnsafeL2:    eth.L2BlockRef{Number: 109},
			},
			prevCurrentL1: eth.BlockRef{Number: 1},
			blocks:        queue.Queue[*types.Block]{block101, block102, block103},
			channels:      []channelStatuser{channel103},
			expected: syncActions{
				blocksToPrune:   3,
				channelsToPrune: 1,
				blocksToLoad:    &inclusiveBlockRange{104, 109},
			},
		},
		{name: "happy path + multiple channels",
			newSyncStatus: eth.SyncStatus{
				HeadL1:      eth.BlockRef{Number: 5},
				CurrentL1:   eth.BlockRef{Number: 2},
				LocalSafeL2: eth.L2BlockRef{Number: 103, Hash: block103.Hash()},
				UnsafeL2:    eth.L2BlockRef{Number: 109},
			},
			prevCurrentL1: eth.BlockRef{Number: 1},
			blocks:        queue.Queue[*types.Block]{block101, block102, block103, block104},
			channels:      []channelStatuser{channel103, channel104},
			expected: syncActions{
				blocksToPrune:   3,
				channelsToPrune: 1,
				blocksToLoad:    &inclusiveBlockRange{105, 109},
			},
			expectedLogs: happyCaseLogs,
		},
		{name: "no progress + unsafe=safe",
			newSyncStatus: eth.SyncStatus{
				HeadL1:      eth.BlockRef{Number: 5},
				CurrentL1:   eth.BlockRef{Number: 2},
				LocalSafeL2: eth.L2BlockRef{Number: 100},
				UnsafeL2:    eth.L2BlockRef{Number: 100},
			},
			prevCurrentL1: eth.BlockRef{Number: 1},
			blocks:        queue.Queue[*types.Block]{},
			channels:      []channelStatuser{},
			expected:      syncActions{},
			expectedLogs:  []string{"no blocks in state"},
		},
		{name: "no progress + unsafe=safe + blocks in state",
			newSyncStatus: eth.SyncStatus{
				HeadL1:      eth.BlockRef{Number: 5},
				CurrentL1:   eth.BlockRef{Number: 2},
				LocalSafeL2: eth.L2BlockRef{Number: 101, Hash: block101.Hash()},
				UnsafeL2:    eth.L2BlockRef{Number: 101},
			},
			prevCurrentL1: eth.BlockRef{Number: 1},
			blocks:        queue.Queue[*types.Block]{block101},
			channels:      []channelStatuser{},
			expected: syncActions{
				blocksToPrune: 1,
			},
			expectedLogs: happyCaseLogs,
		},
	}

	for _, tc := range testCases[len(testCases)-1:] {

		t.Run(tc.name, func(t *testing.T) {
			l, h := testlog.CaptureLogger(t, log.LevelDebug)

			result, outOfSync := computeSyncActions(
				tc.newSyncStatus, tc.prevCurrentL1, tc.blocks, tc.channels, l,
			)

			require.Equal(t, tc.expected, result, "unexpected actions")
			require.Equal(t, tc.expectedSeqOutOfSync, outOfSync)
			if tc.expectedLogs == nil {
				require.Empty(t, h.Logs, "expected no logs but found some", "logs", h.Logs)
			} else {
				for _, e := range tc.expectedLogs {
					r := h.FindLog(testlog.NewMessageContainsFilter(e))
					require.NotNil(t, r, "could not find log message containing '%s'", e)
				}
			}
		})
	}
}
