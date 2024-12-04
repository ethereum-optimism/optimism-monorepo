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

	type TestCase struct {
		name string
		// inputs
		newSyncStatus eth.SyncStatus
		prevCurrentL1 eth.L1BlockRef
		blocks        queue.Queue[*types.Block]
		channels      []channelStatuser
		// expectations
		expected             SyncActions
		expectedSeqOutOfSync bool
		expectedLogs         []string
	}

	testCases := []TestCase{
		{name: "empty sync status",
			newSyncStatus:        eth.SyncStatus{},
			expected:             SyncActions{},
			expectedSeqOutOfSync: true,
			expectedLogs:         []string{"empty sync status"},
		},
		{name: "sequencer restart",
			newSyncStatus: eth.SyncStatus{
				HeadL1:    eth.BlockRef{Number: 2},
				CurrentL1: eth.BlockRef{Number: 1},
			},
			prevCurrentL1:        eth.BlockRef{Number: 2},
			expected:             SyncActions{},
			expectedSeqOutOfSync: true,
			expectedLogs:         []string{"sequencer currentL1 reversed"},
		},
		{name: "L1 reorg",
			newSyncStatus: eth.SyncStatus{
				HeadL1:    eth.BlockRef{Number: 2},
				CurrentL1: eth.BlockRef{Number: 1},
				SafeL2:    eth.L2BlockRef{Number: 100, L1Origin: eth.BlockID{Number: 1}},
				UnsafeL2:  eth.L2BlockRef{Number: 109},
			},
			prevCurrentL1: eth.BlockRef{Number: 1},
			blocks:        queue.Queue[*types.Block]{block102, block103}, // note absence of block101
			channels:      []channelStatuser{channel103},
			expected: SyncActions{
				clearState:   &eth.BlockID{Number: 1},
				blocksToLoad: [2]uint64{101, 109},
			},
			expectedLogs: []string{"new safe head is behind oldest block in state"},
		},
		{name: "batcher restart",
			newSyncStatus: eth.SyncStatus{
				HeadL1:    eth.BlockRef{Number: 2},
				CurrentL1: eth.BlockRef{Number: 2},
				SafeL2:    eth.L2BlockRef{Number: 104, L1Origin: eth.BlockID{Number: 1}},
				UnsafeL2:  eth.L2BlockRef{Number: 109},
			},
			prevCurrentL1: eth.BlockRef{Number: 1},
			blocks:        queue.Queue[*types.Block]{block101, block102, block103},
			channels:      []channelStatuser{channel103},
			expected: SyncActions{
				clearState:   &eth.BlockID{Number: 1},
				blocksToLoad: [2]uint64{105, 109},
			},
			expectedLogs: []string{"safe head above unsafe head"},
		},
		{name: "safe chain reorg",
			newSyncStatus: eth.SyncStatus{
				HeadL1:    eth.BlockRef{Number: 2},
				CurrentL1: eth.BlockRef{Number: 2},
				SafeL2:    eth.L2BlockRef{Number: 103, Hash: block101.Hash(), L1Origin: eth.BlockID{Number: 1}}, // note hash mismatch
				UnsafeL2:  eth.L2BlockRef{Number: 109},
			},
			prevCurrentL1: eth.BlockRef{Number: 1},
			blocks:        queue.Queue[*types.Block]{block101, block102, block103},
			channels:      []channelStatuser{channel103},
			expected: SyncActions{
				clearState:   &eth.BlockID{Number: 1},
				blocksToLoad: [2]uint64{104, 109},
			},
			expectedLogs: []string{"safe chain reorg"},
		},
		{name: "failed to make expected progress",
			newSyncStatus: eth.SyncStatus{
				HeadL1:    eth.BlockRef{Number: 2},
				CurrentL1: eth.BlockRef{Number: 2},
				SafeL2:    eth.L2BlockRef{Number: 101, Hash: block101.Hash(), L1Origin: eth.BlockID{Number: 1}},
				UnsafeL2:  eth.L2BlockRef{Number: 109},
			},
			prevCurrentL1: eth.BlockRef{Number: 1},
			blocks:        queue.Queue[*types.Block]{block101, block102, block103},
			channels:      []channelStatuser{channel103},
			expected: SyncActions{
				clearState:   &eth.BlockID{Number: 1},
				blocksToLoad: [2]uint64{102, 109},
			},
			expectedLogs: []string{"sequencer did not make expected progress"},
		},
		{name: "no progress",
			newSyncStatus: eth.SyncStatus{
				HeadL1:    eth.BlockRef{Number: 1},
				CurrentL1: eth.BlockRef{Number: 1},
				SafeL2:    eth.L2BlockRef{Number: 100},
				UnsafeL2:  eth.L2BlockRef{Number: 109},
			},
			prevCurrentL1: eth.BlockRef{Number: 1},
			blocks:        queue.Queue[*types.Block]{block101, block102, block103},
			channels:      []channelStatuser{channel103},
			expected: SyncActions{
				blocksToLoad: [2]uint64{104, 109},
			},
		},
		{name: "happy path",
			newSyncStatus: eth.SyncStatus{
				HeadL1:    eth.BlockRef{Number: 2},
				CurrentL1: eth.BlockRef{Number: 2},
				SafeL2:    eth.L2BlockRef{Number: 103, Hash: block103.Hash()},
				UnsafeL2:  eth.L2BlockRef{Number: 109},
			},
			prevCurrentL1: eth.BlockRef{Number: 1},
			blocks:        queue.Queue[*types.Block]{block101, block102, block103},
			channels:      []channelStatuser{channel103},
			expected: SyncActions{
				blocksToPrune:   3,
				channelsToPrune: 1,
				blocksToLoad:    [2]uint64{104, 109},
			},
		},
	}

	for _, tc := range testCases {

		t.Run(tc.name, func(t *testing.T) {
			l, h := testlog.CaptureLogger(t, log.LevelDebug)

			result, outOfSync := computeSyncActions(
				tc.newSyncStatus, tc.prevCurrentL1, tc.blocks, tc.channels, l,
			)

			require.Equal(t, tc.expected, result)
			require.Equal(t, tc.expectedSeqOutOfSync, outOfSync)
			for _, e := range tc.expectedLogs {
				r := h.FindLog(testlog.NewMessageContainsFilter(e))
				require.NotNil(t, r, "could not find log message containing '%s'", e)
			}
		})
	}
}
