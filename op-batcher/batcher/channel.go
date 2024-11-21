package batcher

import (
	"math"

	"github.com/ethereum-optimism/optimism/op-batcher/metrics"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
)

// channel is a lightweight wrapper around a ChannelBuilder which keeps track of pending
// and confirmed transactions for a single channel.
type channel struct {
	log  log.Logger
	metr metrics.Metricer
	cfg  ChannelConfig

	// pending channel builder
	channelBuilder *ChannelBuilder
	// Set of unconfirmed txID -> tx data. For tx resubmission
	pendingTransactions map[string]txData
	// Set of confirmed txID -> inclusion block. For determining if the channel is timed out
	confirmedTransactions map[string]eth.BlockID

	// Inclusion block number of first confirmed TX
	minInclusionBlock uint64
	// Inclusion block number of last confirmed TX
	maxInclusionBlock uint64
}

func newChannel(log log.Logger, metr metrics.Metricer, cfg ChannelConfig, rollupCfg *rollup.Config, latestL1OriginBlockNum uint64, channelOut derive.ChannelOut) *channel {
	cb := NewChannelBuilderWithChannelOut(cfg, rollupCfg, latestL1OriginBlockNum, channelOut)
	return &channel{
		log:                   log,
		metr:                  metr,
		cfg:                   cfg,
		channelBuilder:        cb,
		pendingTransactions:   make(map[string]txData),
		confirmedTransactions: make(map[string]eth.BlockID),
		minInclusionBlock:     math.MaxUint64,
	}
}

// TxFailed records a transaction as failed. It will attempt to resubmit the data
// in the failed transaction.
func (c *channel) TxFailed(id string) {
	if data, ok := c.pendingTransactions[id]; ok {
		c.log.Trace("marked transaction as failed", "id", id)
		// Rewind to the first frame of the failed tx
		// -- the frames are ordered, and we want to send them
		// all again.
		c.channelBuilder.RewindFrameCursor(data.Frames()[0])
		delete(c.pendingTransactions, id)
	} else {
		c.log.Warn("unknown transaction marked as failed", "id", id)
	}

	c.metr.RecordBatchTxFailed()
}

// TxConfirmed marks a transaction as confirmed on L1. Returns a bool indicating
// whether the channel timed out on chain.
func (c *channel) TxConfirmed(id string, inclusionBlock eth.BlockID) bool {
	c.metr.RecordBatchTxSuccess()
	c.log.Debug("marked transaction as confirmed", "id", id, "block", inclusionBlock)
	if _, ok := c.pendingTransactions[id]; !ok {
		c.log.Warn("unknown transaction marked as confirmed", "id", id, "block", inclusionBlock)
		// TODO: This can occur if we clear the channel while there are still pending transactions
		// We need to keep track of stale transactions instead
		return false
	}
	delete(c.pendingTransactions, id)
	c.confirmedTransactions[id] = inclusionBlock
	c.channelBuilder.FramePublished(inclusionBlock.Number)

	// Update min/max inclusion blocks for timeout check
	c.minInclusionBlock = min(c.minInclusionBlock, inclusionBlock.Number)
	c.maxInclusionBlock = max(c.maxInclusionBlock, inclusionBlock.Number)

	if c.isFullySubmitted() {
		c.metr.RecordChannelFullySubmitted(c.ID())
		c.log.Info("Channel is fully submitted", "id", c.ID(), "min_inclusion_block", c.minInclusionBlock, "max_inclusion_block", c.maxInclusionBlock)
	}

	// If this channel timed out, put the pending blocks back into the local saved blocks
	// and then reset this state so it can try to build a new channel.
	if c.isTimedOut() {
		c.metr.RecordChannelTimedOut(c.ID())
		c.log.Warn("Channel timed out", "id", c.ID(), "min_inclusion_block", c.minInclusionBlock, "max_inclusion_block", c.maxInclusionBlock)
		return true
	}

	return false
}

// Timeout returns the channel timeout L1 block number. If there is no timeout set, it returns 0.
func (c *channel) Timeout() uint64 {
	return c.channelBuilder.Timeout()
}

// isTimedOut returns true if submitted channel has timed out.
// A channel has timed out if the difference in L1 Inclusion blocks between
// the first & last included block is greater than or equal to the channel timeout.
func (c *channel) isTimedOut() bool {
	// Prior to the granite hard fork activating, the use of the shorter ChannelTimeout here may cause the batcher
	// to believe the channel timed out when it was valid. It would then resubmit the blocks needlessly.
	// This wastes batcher funds but doesn't cause any problems for the chain progressing safe head.
	return len(c.confirmedTransactions) > 0 && c.maxInclusionBlock-c.minInclusionBlock >= c.cfg.ChannelTimeout
}

// isFullySubmitted returns true if the channel has been fully submitted (all transactions are confirmed).
func (c *channel) isFullySubmitted() bool {
	return c.IsFull() && len(c.pendingTransactions)+c.PendingFrames() == 0
}

func (c *channel) NoneSubmitted() bool {
	return len(c.confirmedTransactions) == 0 && len(c.pendingTransactions) == 0
}

func (c *channel) ID() derive.ChannelID {
	return c.channelBuilder.ID()
}

// NextTxData dequeues the next frames from the channel and returns them encoded in a tx data packet.
// If cfg.UseBlobs is false, it returns txData with a single frame.
// If cfg.UseBlobs is true, it will read frames from its channel builder
// until it either doesn't have more frames or the target number of frames is reached.
//
// NextTxData should only be called after HasTxData returned true.
func (c *channel) NextTxData() txData {
	nf := c.cfg.MaxFramesPerTx()
	txdata := txData{frames: make([]frameData, 0, nf), asBlob: c.cfg.UseBlobs}
	for i := 0; i < nf && c.channelBuilder.HasPendingFrame(); i++ {
		frame := c.channelBuilder.NextFrame()
		txdata.frames = append(txdata.frames, frame)
	}

	id := txdata.ID().String()
	c.log.Debug("returning next tx data", "id", id, "num_frames", len(txdata.frames), "as_blob", txdata.asBlob)
	c.pendingTransactions[id] = txdata

	return txdata
}

func (c *channel) HasTxData() bool {
	if c.IsFull() || // If the channel is full, we should start to submit it
		!c.cfg.UseBlobs { // If using calldata, we only send one frame per tx
		return c.channelBuilder.HasPendingFrame()
	}
	// Collect enough frames if channel is not full yet
	return c.channelBuilder.PendingFrames() >= int(c.cfg.MaxFramesPerTx())
}

func (c *channel) IsFull() bool {
	return c.channelBuilder.IsFull()
}

func (c *channel) FullErr() error {
	return c.channelBuilder.FullErr()
}

func (c *channel) CheckTimeout(l1BlockNum uint64) {
	c.channelBuilder.CheckTimeout(l1BlockNum)
}

func (c *channel) AddBlock(block *types.Block) (*derive.L1BlockInfo, error) {
	return c.channelBuilder.AddBlock(block)
}

func (c *channel) InputBytes() int {
	return c.channelBuilder.InputBytes()
}

func (c *channel) ReadyBytes() int {
	return c.channelBuilder.ReadyBytes()
}

func (c *channel) OutputBytes() int {
	return c.channelBuilder.OutputBytes()
}

func (c *channel) TotalFrames() int {
	return c.channelBuilder.TotalFrames()
}

func (c *channel) PendingFrames() int {
	return c.channelBuilder.PendingFrames()
}

func (c *channel) OutputFrames() error {
	return c.channelBuilder.OutputFrames()
}

// LatestL1Origin returns the latest L1 block origin from all the L2 blocks that have been added to the channel
func (c *channel) LatestL1Origin() eth.BlockID {
	return c.channelBuilder.LatestL1Origin()
}

// OldestL1Origin returns the oldest L1 block origin from all the L2 blocks that have been added to the channel
func (c *channel) OldestL1Origin() eth.BlockID {
	return c.channelBuilder.OldestL1Origin()
}

// LatestL2 returns the latest L2 block from all the L2 blocks that have been added to the channel
func (c *channel) LatestL2() eth.BlockID {
	return c.channelBuilder.LatestL2()
}

// OldestL2 returns the oldest L2 block from all the L2 blocks that have been added to the channel
func (c *channel) OldestL2() eth.BlockID {
	return c.channelBuilder.OldestL2()
}

func (c *channel) Close() {
	c.channelBuilder.Close()
}
