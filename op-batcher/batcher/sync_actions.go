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

type inclusiveBlockRange struct{ start, end uint64 }
type syncActions struct {
	clearState      *eth.BlockID
	blocksToPrune   int
	channelsToPrune int
	blocksToLoad    *inclusiveBlockRange // the blocks that should be loaded into the local state.
	// NOTE this range is inclusive on both ends, which is a change to previous behaviour.
}

func (s syncActions) String() string {
	return fmt.Sprintf(
		"SyncActions{blocksToPrune: %d, channelsToPrune: %d, clearState: %v, blocksToLoad: %v}", s.blocksToPrune, s.channelsToPrune, s.clearState, s.blocksToLoad)
}

// computeSyncActions determines the actions that should be taken based on the inputs provided. The inputs are the current
// state of the batcher (blocks and channels), the new sync status, and the previous current L1 block. The actions are returned
// in a struct specifying the number of blocks to prune, the number of channels to prune, whether to wait for node sync, the block
// range to load into the local state, and whether to clear the state entirely. Returns an boolean indicating if the sequencer is out of sync.
func computeSyncActions[T channelStatuser](newSyncStatus eth.SyncStatus, prevCurrentL1 eth.L1BlockRef, blocks queue.Queue[*types.Block], channels []T, l log.Logger) (syncActions, bool) {

	// PART 1: Initial checks on the sync status
	if newSyncStatus.HeadL1 == (eth.L1BlockRef{}) {
		l.Warn("empty sync status")
		return syncActions{}, true
	}

	if newSyncStatus.CurrentL1.Number < prevCurrentL1.Number {
		// This can happen when the sequencer restarts
		l.Warn("sequencer currentL1 reversed")
		return syncActions{}, true
	}

	var allUnsafeBlocks *inclusiveBlockRange
	if newSyncStatus.UnsafeL2.Number > newSyncStatus.LocalSafeL2.Number {
		allUnsafeBlocks = &inclusiveBlockRange{newSyncStatus.LocalSafeL2.Number + 1, newSyncStatus.UnsafeL2.Number}
	}

	// PART 2: checks involving only the oldest block in the state
	oldestBlockInState, hasBlocks := blocks.Peek()

	if !hasBlocks {
		s := syncActions{
			blocksToLoad: allUnsafeBlocks,
		}
		l.Info("no blocks in state", "syncActions", s)
		return s, false
	}

	// These actions apply in multiple unhappy scenarios below, where
	// we detect that the existing state is invalidated
	// and we need to start over, loading all unsafe blocks
	// from the sequencer.
	startAfresh := syncActions{
		clearState:   &newSyncStatus.LocalSafeL2.L1Origin,
		blocksToLoad: allUnsafeBlocks,
	}

	oldestBlockInStateNum := oldestBlockInState.NumberU64()
	nextSafeBlockNum := newSyncStatus.LocalSafeL2.Number + 1

	if nextSafeBlockNum < oldestBlockInStateNum {
		l.Warn("next safe block is below oldest block in state",
			"syncActions", startAfresh,
			"oldestBlockInState", oldestBlockInState,
			"safeL2", newSyncStatus.LocalSafeL2)
		return startAfresh, false
	}

	// PART 3: checks involving all blocks in state
	newestBlockInState := blocks[blocks.Len()-1]
	newestBlockInStateNum := newestBlockInState.NumberU64()

	numBlocksToDequeue := nextSafeBlockNum - oldestBlockInStateNum

	if numBlocksToDequeue > uint64(blocks.Len()) {
		// This could happen if the batcher restarted.
		// The sequencer may have derived the safe chain
		// from channels sent by a previous batcher instance.
		l.Warn("safe head above newest block in state, clearing channel manager state",
			"syncActions", startAfresh,
			"safeL2", newSyncStatus.LocalSafeL2,
			"newestBlockInState", eth.ToBlockID(newestBlockInState),
		)
		return startAfresh, false
	}

	if numBlocksToDequeue > 0 && blocks[numBlocksToDequeue-1].Hash() != newSyncStatus.LocalSafeL2.Hash {
		l.Warn("safe chain reorg, clearing channel manager state",
			"syncActions", startAfresh,
			"existingBlock", eth.ToBlockID(blocks[numBlocksToDequeue-1]),
			"safeL2", newSyncStatus.LocalSafeL2)
		return startAfresh, false
	}

	// PART 4: checks involving channels
	for _, ch := range channels {
		if ch.isFullySubmitted() &&
			!ch.isTimedOut() &&
			newSyncStatus.CurrentL1.Number > ch.MaxInclusionBlock() &&
			newSyncStatus.LocalSafeL2.Number < ch.LatestL2().Number {
			// Safe head did not make the expected progress
			// for a fully submitted channel. This indicates
			// that the derivation pipeline may have stalled
			// e.g. because of Holocene strict ordering rules.
			l.Warn("sequencer did not make expected progress",
				"syncActions", startAfresh,
				"existingBlock", ch.LatestL2(),
				"safeL2", newSyncStatus.LocalSafeL2)
			return startAfresh, false
		}
	}

	// PART 5: happy path
	numChannelsToPrune := 0
	for _, ch := range channels {
		if ch.LatestL2().Number > newSyncStatus.LocalSafeL2.Number {
			// If the channel has blocks which are not yet safe
			// we do not want to prune it.
			break
		}
		numChannelsToPrune++
	}

	var allUnsafeBlocksAboveState *inclusiveBlockRange
	if newSyncStatus.UnsafeL2.Number > newestBlockInStateNum {
		allUnsafeBlocksAboveState = &inclusiveBlockRange{newestBlockInStateNum + 1, newSyncStatus.UnsafeL2.Number}
	}

	return syncActions{
		blocksToPrune:   int(numBlocksToDequeue),
		channelsToPrune: numChannelsToPrune,
		blocksToLoad:    allUnsafeBlocksAboveState,
	}, false
}
