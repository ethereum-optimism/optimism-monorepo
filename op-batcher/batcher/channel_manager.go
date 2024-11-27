package batcher

import (
	"errors"
	"fmt"
	"io"
	"math"
	"sync"

	"github.com/ethereum-optimism/optimism/op-batcher/metrics"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/queue"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
)

var ErrReorg = errors.New("block does not extend existing chain")

type ChannelOutFactory func(cfg ChannelConfig, rollupCfg *rollup.Config) (derive.ChannelOut, error)

// channelManager stores a contiguous set of blocks & turns them into channels.
// Upon receiving tx confirmation (or a tx failure), it does channel error handling.
//
// For simplicity, it only creates a single pending channel at a time & waits for
// the channel to either successfully be submitted or timeout before creating a new
// channel.
// Public functions on channelManager are safe for concurrent access.
type channelManager struct {
	mu          sync.Mutex
	log         log.Logger
	metr        metrics.Metricer
	cfgProvider ChannelConfigProvider
	rollupCfg   *rollup.Config

	outFactory ChannelOutFactory

	// All blocks since the last request for new tx data.
	blocks queue.Queue[*types.Block]
	// blockCursor is an index into blocks queue. It points at the next block
	// to build a channel with. blockCursor = len(blocks) is reserved for when
	// there are no blocks ready to build with.
	blockCursor int
	// The latest L1 block from all the L2 blocks in the most recently submitted channel.
	// Used to track channel duration timeouts.
	l1OriginLastSubmittedChannel eth.BlockID
	// The default ChannelConfig to use for the next channel
	defaultCfg ChannelConfig
	// last block hash - for reorg detection
	tip common.Hash

	// channel to write new block data to
	currentChannel *channel
	// channels to read frame data from, for writing batches onchain
	channelQueue []*channel
	// used to lookup channels by tx ID upon tx success / failure
	txChannels map[string]*channel
}

func NewChannelManager(log log.Logger, metr metrics.Metricer, cfgProvider ChannelConfigProvider, rollupCfg *rollup.Config) *channelManager {
	return &channelManager{
		log:         log,
		metr:        metr,
		cfgProvider: cfgProvider,
		defaultCfg:  cfgProvider.ChannelConfig(),
		rollupCfg:   rollupCfg,
		outFactory:  NewChannelOut,
		txChannels:  make(map[string]*channel),
	}
}

func (s *channelManager) SetChannelOutFactory(outFactory ChannelOutFactory) {
	s.outFactory = outFactory
}

// Clear clears the entire state of the channel manager.
// It is intended to be used before launching op-batcher and after an L2 reorg.
func (s *channelManager) Clear(l1OriginLastSubmittedChannel eth.BlockID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.log.Trace("clearing channel manager state")
	s.blocks.Clear()
	s.blockCursor = 0
	s.l1OriginLastSubmittedChannel = l1OriginLastSubmittedChannel
	s.tip = common.Hash{}
	s.currentChannel = nil
	s.channelQueue = nil
	s.txChannels = make(map[string]*channel)
}

func (s *channelManager) pendingBlocks() int {
	return s.blocks.Len() - s.blockCursor
}

// TxFailed records a transaction as failed. It will attempt to resubmit the data
// in the failed transaction.
func (s *channelManager) TxFailed(_id txID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := _id.String()
	if channel, ok := s.txChannels[id]; ok {
		delete(s.txChannels, id)
		channel.TxFailed(id)
	} else {
		s.log.Warn("transaction from unknown channel marked as failed", "id", id)
	}
}

// TxConfirmed marks a transaction as confirmed on L1. Only if the channel timed out
// the channelManager's state is modified.
func (s *channelManager) TxConfirmed(_id txID, inclusionBlock eth.BlockID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := _id.String()
	if channel, ok := s.txChannels[id]; ok {
		delete(s.txChannels, id)
		if timedOut := channel.TxConfirmed(id, inclusionBlock); timedOut {
			s.handleChannelInvalidated(channel)
		}
	} else {
		s.log.Warn("transaction from unknown channel marked as confirmed", "id", id)
	}
	s.metr.RecordBatchTxSubmitted()
	s.log.Debug("marked transaction as confirmed", "id", id, "block", inclusionBlock)
}

// rewindToBlock updates the blockCursor to point at
// the block with the supplied hash, only if that block exists
// in the block queue and the blockCursor is ahead of it.
// Panics if the block is not in state.
func (s *channelManager) rewindToBlock(block eth.BlockID) {
	idx := block.Number - s.blocks[0].Number().Uint64()
	if s.blocks[idx].Hash() == block.Hash && idx < uint64(s.blockCursor) {
		s.blockCursor = int(idx)
	} else {
		panic("tried to rewind to nonexistent block")
	}
}

// handleChannelInvalidated rewinds the channelManager's blockCursor
// to point at the first block added to the provided channel,
// and removes the channel from the channelQueue, along with
// any channels which are newer than the provided channel.
func (s *channelManager) handleChannelInvalidated(c *channel) {
	if len(c.channelBuilder.blocks) > 0 {
		// This is usually true, but there is an edge case
		// where a channel timed out before any blocks got added.
		// In that case we end up with an empty frame (header only),
		// and there are no blocks to requeue.
		blockID := eth.ToBlockID(c.channelBuilder.blocks[0])
		for _, block := range c.channelBuilder.blocks {
			s.metr.RecordL2BlockInPendingQueue(block)
		}
		s.rewindToBlock(blockID)
	} else {
		s.log.Debug("channelManager.handleChanneInvalidated: channel had no blocks")
	}

	// Trim provided channel and any older channels:
	for i := range s.channelQueue {
		if s.channelQueue[i] == c {
			s.channelQueue = s.channelQueue[:i]
			break
		}
	}

	// We want to start writing to a new channel, so reset currentChannel.
	s.currentChannel = nil
}

// nextTxData dequeues frames from the channel and returns them encoded in a transaction.
// It also updates the internal tx -> channels mapping
func (s *channelManager) nextTxData(channel *channel) (txData, error) {
	if channel == nil || !channel.HasTxData() {
		s.log.Trace("no next tx data")
		return txData{}, io.EOF // TODO: not enough data error instead
	}
	tx := channel.NextTxData()

	// update s.l1OriginLastSubmittedChannel so that the next
	// channel's duration timeout will trigger properly
	if channel.LatestL1Origin().Number > s.l1OriginLastSubmittedChannel.Number {
		s.l1OriginLastSubmittedChannel = channel.LatestL1Origin()
	}
	s.txChannels[tx.ID().String()] = channel
	return tx, nil
}

// TxData returns the next tx data that should be submitted to L1.
//
// If the current channel is
// full, it only returns the remaining frames of this channel until it got
// successfully fully sent to L1. It returns io.EOF if there's no pending tx data.
//
// It will decide whether to switch DA type automatically.
// When switching DA type, the channelManager state will be rebuilt
// with a new ChannelConfig.
func (s *channelManager) TxData(l1Head eth.BlockID) (txData, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	channel, err := s.getReadyChannel(l1Head)
	if err != nil {
		return emptyTxData, err
	}
	// If the channel has already started being submitted,
	// return now and ensure no requeueing happens
	if !channel.NoneSubmitted() {
		return s.nextTxData(channel)
	}

	// Call provider method to reassess optimal DA type
	newCfg := s.cfgProvider.ChannelConfig()

	// No change:
	if newCfg.UseBlobs == s.defaultCfg.UseBlobs {
		s.log.Debug("Recomputing optimal ChannelConfig: no need to switch DA type",
			"useBlobs", s.defaultCfg.UseBlobs)
		return s.nextTxData(channel)
	}

	// Change:
	s.log.Info("Recomputing optimal ChannelConfig: changing DA type and requeing blocks...",
		"useBlobsBefore", s.defaultCfg.UseBlobs,
		"useBlobsAfter", newCfg.UseBlobs)

	// Invalidate the channel so its blocks
	// get requeued:
	s.handleChannelInvalidated(channel)

	// Set the defaultCfg so new channels
	// pick up the new ChannelConfig
	s.defaultCfg = newCfg

	// Try again to get data to send on chain.
	channel, err = s.getReadyChannel(l1Head)
	if err != nil {
		return emptyTxData, err
	}
	return s.nextTxData(channel)
}

// getReadyChannel returns the next channel ready to submit data, or an error.
// It will create a new channel if necessary.
// If there is no data ready to send, it adds blocks from the block queue
// to the current channel and generates frames for it.
// Always returns nil and the io.EOF sentinel error when
// there is no channel with txData
func (s *channelManager) getReadyChannel(l1Head eth.BlockID) (*channel, error) {
	var firstWithTxData *channel
	for _, ch := range s.channelQueue {
		if ch.HasTxData() {
			firstWithTxData = ch
			break
		}
	}

	dataPending := firstWithTxData != nil
	s.log.Debug("Requested tx data", "l1Head", l1Head, "txdata_pending", dataPending, "blocks_pending", s.blocks.Len())

	// Short circuit if there is pending tx data or the channel manager is closed
	if dataPending {
		return firstWithTxData, nil
	}

	// No pending tx data, so we have to add new blocks to the channel
	// If we have no saved blocks, we will not be able to create valid frames
	if s.pendingBlocks() == 0 {
		return nil, io.EOF
	}

	if err := s.ensureChannelWithSpace(l1Head); err != nil {
		return nil, err
	}

	if err := s.processBlocks(); err != nil {
		return nil, err
	}

	// Register current L1 head only after all pending blocks have been
	// processed. Even if a timeout will be triggered now, it is better to have
	// all pending blocks be included in this channel for submission.
	s.registerL1Block(l1Head)

	if err := s.outputFrames(); err != nil {
		return nil, err
	}

	if s.currentChannel.HasTxData() {
		return s.currentChannel, nil
	}

	return nil, io.EOF
}

// ensureChannelWithSpace ensures currentChannel is populated with a channel that has
// space for more data (i.e. channel.IsFull returns false). If currentChannel is nil
// or full, a new channel is created.
func (s *channelManager) ensureChannelWithSpace(l1Head eth.BlockID) error {
	if s.currentChannel != nil && !s.currentChannel.IsFull() {
		return nil
	}

	// We reuse the ChannelConfig from the last channel.
	// This will be reassessed at channel submission-time,
	// but this is our best guess at the appropriate values for now.
	cfg := s.defaultCfg

	channelOut, err := s.outFactory(cfg, s.rollupCfg)
	if err != nil {
		return fmt.Errorf("creating channel out: %w", err)
	}

	pc := newChannel(s.log, s.metr, cfg, s.rollupCfg, s.l1OriginLastSubmittedChannel.Number, channelOut)

	s.currentChannel = pc
	s.channelQueue = append(s.channelQueue, pc)

	s.log.Info("Created channel",
		"id", pc.ID(),
		"l1Head", l1Head,
		"blocks_pending", s.pendingBlocks(),
		"l1OriginLastSubmittedChannel", s.l1OriginLastSubmittedChannel,
		"batch_type", cfg.BatchType,
		"compression_algo", cfg.CompressorConfig.CompressionAlgo,
		"target_num_frames", cfg.TargetNumFrames,
		"max_frame_size", cfg.MaxFrameSize,
		"use_blobs", cfg.UseBlobs,
	)
	s.metr.RecordChannelOpened(pc.ID(), s.blocks.Len())

	return nil
}

// registerL1Block registers the given block at the current channel.
func (s *channelManager) registerL1Block(l1Head eth.BlockID) {
	s.currentChannel.CheckTimeout(l1Head.Number)
	s.log.Debug("new L1-block registered at channel builder",
		"l1Head", l1Head,
		"channel_full", s.currentChannel.IsFull(),
		"full_reason", s.currentChannel.FullErr(),
	)
}

// processBlocks adds blocks from the blocks queue to the current channel until
// either the queue got exhausted or the channel is full.
func (s *channelManager) processBlocks() error {
	var (
		blocksAdded int
		_chFullErr  *ChannelFullError // throw away, just for type checking
		latestL2ref eth.L2BlockRef
	)

	for i := s.blockCursor; ; i++ {
		block, ok := s.blocks.PeekN(i)
		if !ok {
			break
		}

		l1info, err := s.currentChannel.AddBlock(block)
		if errors.As(err, &_chFullErr) {
			// current block didn't get added because channel is already full
			break
		} else if err != nil {
			return fmt.Errorf("adding block[%d] to channel builder: %w", i, err)
		}
		s.log.Debug("Added block to channel", "id", s.currentChannel.ID(), "block", eth.ToBlockID(block))

		blocksAdded += 1
		latestL2ref = l2BlockRefFromBlockAndL1Info(block, l1info)
		s.metr.RecordL2BlockInChannel(block)
		// current block got added but channel is now full
		if s.currentChannel.IsFull() {
			break
		}
	}

	s.blockCursor += blocksAdded

	s.metr.RecordL2BlocksAdded(latestL2ref,
		blocksAdded,
		s.blocks.Len(),
		s.currentChannel.InputBytes(),
		s.currentChannel.ReadyBytes())
	s.log.Debug("Added blocks to channel",
		"blocks_added", blocksAdded,
		"blocks_pending", s.pendingBlocks(),
		"channel_full", s.currentChannel.IsFull(),
		"input_bytes", s.currentChannel.InputBytes(),
		"ready_bytes", s.currentChannel.ReadyBytes(),
	)
	return nil
}

// outputFrames generates frames for the current channel, and computes and logs the compression ratio
func (s *channelManager) outputFrames() error {
	if err := s.currentChannel.OutputFrames(); err != nil {
		return fmt.Errorf("creating frames with channel builder: %w", err)
	}
	if !s.currentChannel.IsFull() {
		return nil
	}

	inBytes, outBytes := s.currentChannel.InputBytes(), s.currentChannel.OutputBytes()
	s.metr.RecordChannelClosed(
		s.currentChannel.ID(),
		s.pendingBlocks(),
		s.currentChannel.TotalFrames(),
		inBytes,
		outBytes,
		s.currentChannel.FullErr(),
	)

	var comprRatio float64
	if inBytes > 0 {
		comprRatio = float64(outBytes) / float64(inBytes)
	}

	s.log.Info("Channel closed",
		"id", s.currentChannel.ID(),
		"blocks_pending", s.pendingBlocks(),
		"num_frames", s.currentChannel.TotalFrames(),
		"input_bytes", inBytes,
		"output_bytes", outBytes,
		"oldest_l1_origin", s.currentChannel.OldestL1Origin(),
		"l1_origin", s.currentChannel.LatestL1Origin(),
		"oldest_l2", s.currentChannel.OldestL2(),
		"latest_l2", s.currentChannel.LatestL2(),
		"full_reason", s.currentChannel.FullErr(),
		"compr_ratio", comprRatio,
	)
	return nil
}

// AddL2Block adds an L2 block to the internal blocks queue. It returns ErrReorg
// if the block does not extend the last block loaded into the state. If no
// blocks were added yet, the parent hash check is skipped.
func (s *channelManager) AddL2Block(block *types.Block) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.tip != (common.Hash{}) && s.tip != block.ParentHash() {
		return ErrReorg
	}

	s.metr.RecordL2BlockInPendingQueue(block)
	s.blocks.Enqueue(block)
	s.tip = block.Hash()

	return nil
}

func l2BlockRefFromBlockAndL1Info(block *types.Block, l1info *derive.L1BlockInfo) eth.L2BlockRef {
	return eth.L2BlockRef{
		Hash:           block.Hash(),
		Number:         block.NumberU64(),
		ParentHash:     block.ParentHash(),
		Time:           block.Time(),
		L1Origin:       eth.BlockID{Hash: l1info.BlockHash, Number: l1info.Number},
		SequenceNumber: l1info.SequenceNumber,
	}
}

var ErrPendingAfterClose = errors.New("pending channels remain after closing channel-manager")

// pruneSafeBlocks dequeues blocks from the internal blocks queue
// if they have now become safe.
func (s *channelManager) pruneSafeBlocks(newSafeHead eth.L2BlockRef) {
	oldestBlock, ok := s.blocks.Peek()
	if !ok {
		// no blocks to prune
		return
	}

	if newSafeHead.Number+1 == oldestBlock.NumberU64() {
		// no blocks to prune
		return
	}

	if newSafeHead.Number+1 < oldestBlock.NumberU64() {
		// This could happen if there was an L1 reorg.
		// Or if the sequencer restarted.
		s.log.Warn("safe head reversed, clearing channel manager state",
			"oldestBlock", eth.ToBlockID(oldestBlock),
			"newSafeBlock", newSafeHead)
		// We should restart work from the new safe head,
		// and therefore prune all the blocks.
		s.Clear(newSafeHead.L1Origin)
		return
	}

	numBlocksToDequeue := newSafeHead.Number + 1 - oldestBlock.NumberU64()

	if numBlocksToDequeue > uint64(s.blocks.Len()) {
		// This could happen if the batcher restarted.
		// The sequencer may have derived the safe chain
		// from channels sent by a previous batcher instance.
		s.log.Warn("safe head above unsafe head, clearing channel manager state",
			"unsafeBlock", eth.ToBlockID(s.blocks[s.blocks.Len()-1]),
			"newSafeBlock", newSafeHead)
		// We should restart work from the new safe head,
		// and therefore prune all the blocks.
		s.Clear(newSafeHead.L1Origin)
		return
	}

	if s.blocks[numBlocksToDequeue-1].Hash() != newSafeHead.Hash {
		s.log.Warn("safe chain reorg, clearing channel manager state",
			"existingBlock", eth.ToBlockID(s.blocks[numBlocksToDequeue-1]),
			"newSafeBlock", newSafeHead)
		// We should restart work from the new safe head,
		// and therefore prune all the blocks.
		s.Clear(newSafeHead.L1Origin)
		return
	}

	// This shouldn't return an error because
	// We already checked numBlocksToDequeue <= s.blocks.Len()
	_, _ = s.blocks.DequeueN(int(numBlocksToDequeue))
	s.blockCursor -= int(numBlocksToDequeue)

	if s.blockCursor < 0 {
		panic("negative blockCursor")
	}
}

// pruneChannels dequeues channels from the internal channels queue
// if they were built using blocks which are now safe
func (s *channelManager) pruneChannels(newSafeHead eth.L2BlockRef) {
	i := 0
	for _, ch := range s.channelQueue {
		if ch.LatestL2().Number > newSafeHead.Number {
			break
		}
		i++
	}
	s.channelQueue = s.channelQueue[i:]
}

// PendingDABytes returns the current number of bytes pending to be written to the DA layer (from blocks fetched from L2
// but not yet in a channel).
func (s *channelManager) PendingDABytes() int64 {
	f := s.metr.PendingDABytes()
	if f >= math.MaxInt64 {
		return math.MaxInt64
	}
	if f <= math.MinInt64 {
		return math.MinInt64
	}
	return int64(f)
}

// CheckExpectedProgress uses the supplied syncStatus to infer
// whether the node providing the status has made the expected
// safe head progress given fully submitted channels held in
// state.
func (m *channelManager) CheckExpectedProgress(syncStatus eth.SyncStatus) error {
	for _, ch := range m.channelQueue {
		if ch.isFullySubmitted() && // This implies a number of l1 confirmations has passed, depending on how the txmgr was configured
			!ch.isTimedOut() &&
			syncStatus.CurrentL1.Number > ch.maxInclusionBlock &&
			syncStatus.SafeL2.Number < ch.LatestL2().Number {
			return errors.New("safe head did not make expected progress")
		}
	}
	return nil
}

func (m *channelManager) LastStoredBlock() eth.BlockID {
	if m.blocks.Len() == 0 {
		return eth.BlockID{}
	}
	return eth.ToBlockID(m.blocks[m.blocks.Len()-1])
}
