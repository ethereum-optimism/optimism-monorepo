package batcher

import (
	"fmt"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/queue"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
)

type channelStatuser interface {
	isFullySubmitted() bool
	isTimedOut() bool
	LatestL2() eth.BlockID
	MaxInclusionBlock() uint64
}

type syncActions struct {
	clearState      *eth.BlockID
	blocksToPrune   int
	channelsToPrune int
	blocksToLoad    [2]uint64 // the range [start,end] that should be loaded into the local state.
	// NOTE this range is inclusive on both ends, which is a change to previous behaviour.
}

func (s syncActions) String() string {
	return fmt.Sprintf(
		"SyncActions{blocksToPrune: %d, channelsToPrune: %d, clearState: %v, blocksToLoad: [%d, %d]}", s.blocksToPrune, s.channelsToPrune, s.clearState, s.blocksToLoad[0], s.blocksToLoad[1])
}

// computeSyncActions determines the actions that should be taken based on the inputs provided. The inputs are the current
// state of the batcher (blocks and channels), the new sync status, and the previous current L1 block. The actions are returned
// in a struct specifying the number of blocks to prune, the number of channels to prune, whether to wait for node sync, the block
// range to load into the local state, and whether to clear the state entirely. Returns an boolean indicating if the sequencer is out of sync.
func computeSyncActions[T channelStatuser](newSyncStatus eth.SyncStatus, prevCurrentL1 eth.L1BlockRef, blocks queue.Queue[*types.Block], channels []T, l log.Logger) (syncActions, bool) {

	if newSyncStatus.HeadL1 == (eth.L1BlockRef{}) {
		l.Warn("empty sync status")
		return syncActions{}, true
	}

	if newSyncStatus.CurrentL1.Number < prevCurrentL1.Number {
		// This can happen when the sequencer restarts
		l.Warn("sequencer currentL1 reversed")
		return syncActions{}, true
	}

	oldestBlockInState, hasBlocks := blocks.Peek()
	oldestBlockInStateNum := oldestBlockInState.NumberU64()

	oldestUnsafeBlockNum := newSyncStatus.SafeL2.Number + 1
	youngestUnsafeBlockNum := newSyncStatus.UnsafeL2.Number

	if !hasBlocks {
		s := syncActions{
			blocksToLoad: [2]uint64{oldestUnsafeBlockNum, youngestUnsafeBlockNum},
		}
		l.Info("no blocks in state", "syncActions", s)
		return s, false
	}

	if oldestUnsafeBlockNum < oldestBlockInStateNum {
		s := syncActions{
			clearState:   &newSyncStatus.SafeL2.L1Origin,
			blocksToLoad: [2]uint64{oldestUnsafeBlockNum, youngestUnsafeBlockNum},
		}
		l.Warn("new safe head is behind oldest block in state", "syncActions", s)
		return s, false
	}

	newestBlockInState := blocks[blocks.Len()-1]
	newestBlockInStateNum := newestBlockInState.NumberU64()

	numBlocksToDequeue := oldestUnsafeBlockNum - oldestBlockInStateNum

	if numBlocksToDequeue > uint64(blocks.Len()) {
		// This could happen if the batcher restarted.
		// The sequencer may have derived the safe chain
		// from channels sent by a previous batcher instance.
		s := syncActions{
			clearState:   &newSyncStatus.SafeL2.L1Origin,
			blocksToLoad: [2]uint64{oldestUnsafeBlockNum, youngestUnsafeBlockNum},
		}
		l.Warn("safe head above unsafe head, clearing channel manager state",
			"unsafeBlock", eth.ToBlockID(newestBlockInState),
			"newSafeBlock", newSyncStatus.SafeL2.Number,
			"syncActions",
			s)
		return s, false
	}

	if numBlocksToDequeue > 0 && blocks[numBlocksToDequeue-1].Hash() != newSyncStatus.SafeL2.Hash {
		s := syncActions{
			clearState:   &newSyncStatus.SafeL2.L1Origin,
			blocksToLoad: [2]uint64{oldestUnsafeBlockNum, youngestUnsafeBlockNum},
		}
		l.Warn("safe chain reorg, clearing channel manager state",
			"existingBlock", eth.ToBlockID(blocks[numBlocksToDequeue-1]),
			"newSafeBlock", newSyncStatus.SafeL2,
			"syncActions", s)
		// We should resume work from the new safe head,
		// and therefore prune all the blocks.
		return s, false
	}

	for _, ch := range channels {
		if ch.isFullySubmitted() &&
			!ch.isTimedOut() &&
			newSyncStatus.CurrentL1.Number > ch.MaxInclusionBlock() &&
			newSyncStatus.SafeL2.Number < ch.LatestL2().Number {

			s := syncActions{
				clearState:   &newSyncStatus.SafeL2.L1Origin,
				blocksToLoad: [2]uint64{oldestUnsafeBlockNum, youngestUnsafeBlockNum},
			}
			// Safe head did not make the expected progress
			// for a fully submitted channel. We should go back to
			// the last safe head and resume work from there.
			l.Warn("sequencer did not make expected progress",
				"existingBlock", eth.ToBlockID(blocks[numBlocksToDequeue-1]),
				"newSafeBlock", newSyncStatus.SafeL2,
				"syncActions", s)
			return s, false
		}
	}

	numChannelsToPrune := 0
	for _, ch := range channels {
		if ch.LatestL2().Number > newSyncStatus.SafeL2.Number {
			break
		}
		numChannelsToPrune++
	}

	start := newestBlockInStateNum + 1
	end := youngestUnsafeBlockNum

	// happy path
	return syncActions{
		blocksToPrune:   int(numBlocksToDequeue),
		channelsToPrune: numChannelsToPrune,
		blocksToLoad:    [2]uint64{start, end},
	}, false
}
