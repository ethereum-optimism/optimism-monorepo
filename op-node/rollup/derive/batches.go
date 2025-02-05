package derive

import (
	"bytes"
	"context"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
)

type BatchWithL1InclusionBlock struct {
	Batch
	L1InclusionBlock eth.L1BlockRef
}

type BatchValidity uint8

const (
	// BatchDrop indicates that the batch is invalid, and will always be in the future, unless we reorg
	BatchDrop = iota
	// BatchAccept indicates that the batch is valid and should be processed
	BatchAccept
	// BatchUndecided indicates we are lacking L1 information until we can proceed batch filtering
	BatchUndecided
	// BatchFuture indicates that the batch may be valid, but cannot be processed yet and should be checked again later
	BatchFuture
	// BatchPast indicates that the batch is from the past, i.e. its timestamp is smaller or equal
	// to the safe head's timestamp.
	BatchPast
)

// CheckBatch checks if the given batch can be applied on top of the given l2SafeHead, given the contextual L1 blocks the batch was included in.
// The first entry of the l1Blocks should match the origin of the l2SafeHead. One or more consecutive l1Blocks should be provided.
// In case of only a single L1 block, the decision whether a batch is valid may have to stay undecided.
func CheckBatch(ctx context.Context, cfg *rollup.Config, log log.Logger, l1Blocks []eth.L1BlockRef,
	l2SafeHead eth.L2BlockRef, batch *BatchWithL1InclusionBlock, l2Fetcher SafeBlockFetcher,
) BatchValidity {
	switch typ := batch.GetBatchType(); typ {
	case SingularBatchType:
		singularBatch, ok := batch.AsSingularBatch()
		if !ok {
			log.Error("failed type assertion to SingularBatch")
			return BatchDrop
		}
		return checkSingularBatch(cfg, log, l1Blocks, l2SafeHead, singularBatch, batch.L1InclusionBlock)
	case SpanBatchType:
		spanBatch, ok := batch.AsSpanBatch()
		if !ok {
			log.Error("failed type assertion to SpanBatch")
			return BatchDrop
		}
		return checkSpanBatch(ctx, cfg, log, l1Blocks, l2SafeHead, spanBatch, batch.L1InclusionBlock, l2Fetcher)
	default:
		log.Warn("Unrecognized batch type: %d", typ)
		return BatchDrop
	}
}

// checkSingularBatch implements SingularBatch validation rule.
func checkSingularBatch(cfg *rollup.Config, log log.Logger, l1Blocks []eth.L1BlockRef, l2SafeHead eth.L2BlockRef, batch *SingularBatch, l1InclusionBlock eth.L1BlockRef) BatchValidity {
	// add details to the log
	log = batch.LogContext(log)

	// sanity check we have consistent inputs
	if len(l1Blocks) == 0 {
		log.Warn("missing L1 block input, cannot proceed with batch checking")
		return BatchUndecided
	}
	epoch := l1Blocks[0]

	nextTimestamp := l2SafeHead.Time + cfg.BlockTime
	if batch.Timestamp > nextTimestamp {
		if cfg.IsHolocene(l1InclusionBlock.Time) {
			log.Warn("dropping future batch", "next_timestamp", nextTimestamp)
			return BatchDrop
		}
		log.Trace("received out-of-order batch for future processing after next batch", "next_timestamp", nextTimestamp)
		return BatchFuture
	}
	if batch.Timestamp < nextTimestamp {
		log.Warn("dropping past batch with old timestamp", "min_timestamp", nextTimestamp)
		if cfg.IsHolocene(l1InclusionBlock.Time) {
			return BatchPast
		}
		return BatchDrop
	}

	// dependent on above timestamp check. If the timestamp is correct, then it must build on top of the safe head.
	if batch.ParentHash != l2SafeHead.Hash {
		log.Warn("ignoring batch with mismatching parent hash", "current_safe_head", l2SafeHead.Hash)
		return BatchDrop
	}

	// Filter out batches that were included too late.
	if uint64(batch.EpochNum)+cfg.SeqWindowSize < l1InclusionBlock.Number {
		log.Warn("batch was included too late, sequence window expired")
		return BatchDrop
	}

	// Check the L1 origin of the batch
	batchOrigin := epoch
	if uint64(batch.EpochNum) < epoch.Number {
		log.Warn("dropped batch, epoch is too old", "minimum", epoch.ID())
		// batch epoch too old
		return BatchDrop
	} else if uint64(batch.EpochNum) == epoch.Number {
		// Batch is sticking to the current epoch, continue.
	} else if uint64(batch.EpochNum) == epoch.Number+1 {
		// With only 1 l1Block we cannot look at the next L1 Origin.
		// Note: This means that we are unable to determine validity of a batch
		// without more information. In this case we should bail out until we have
		// more information otherwise the eager algorithm may diverge from a non-eager
		// algorithm.
		if len(l1Blocks) < 2 {
			log.Info("eager batch wants to advance epoch, but could not without more L1 blocks", "current_epoch", epoch.ID())
			return BatchUndecided
		}
		batchOrigin = l1Blocks[1]
	} else {
		log.Warn("batch is for future epoch too far ahead, while it has the next timestamp, so it must be invalid", "current_epoch", epoch.ID())
		return BatchDrop
	}

	if batch.EpochHash != batchOrigin.Hash {
		log.Warn("batch is for different L1 chain, epoch hash does not match", "expected", batchOrigin.ID())
		return BatchDrop
	}

	if batch.Timestamp < batchOrigin.Time {
		log.Warn("batch timestamp is less than L1 origin timestamp", "l2_timestamp", batch.Timestamp, "l1_timestamp", batchOrigin.Time, "origin", batchOrigin.ID())
		return BatchDrop
	}

	spec := rollup.NewChainSpec(cfg)
	// Check if we ran out of sequencer time drift
	if max := batchOrigin.Time + spec.MaxSequencerDrift(batchOrigin.Time); batch.Timestamp > max {
		if len(batch.Transactions) == 0 {
			// If the sequencer is co-operating by producing an empty batch,
			// then allow the batch if it was the right thing to do to maintain the L2 time >= L1 time invariant.
			// We only check batches that do not advance the epoch, to ensure epoch advancement regardless of time drift is allowed.
			if epoch.Number == batchOrigin.Number {
				if len(l1Blocks) < 2 {
					log.Info("without the next L1 origin we cannot determine yet if this empty batch that exceeds the time drift is still valid")
					return BatchUndecided
				}
				nextOrigin := l1Blocks[1]
				if batch.Timestamp >= nextOrigin.Time { // check if the next L1 origin could have been adopted
					log.Info("batch exceeded sequencer time drift without adopting next origin, and next L1 origin would have been valid")
					return BatchDrop
				} else {
					log.Info("continuing with empty batch before late L1 block to preserve L2 time invariant")
				}
			}
		} else {
			// If the sequencer is ignoring the time drift rule, then drop the batch and force an empty batch instead,
			// as the sequencer is not allowed to include anything past this point without moving to the next epoch.
			log.Warn("batch exceeded sequencer time drift, sequencer must adopt new L1 origin to include transactions again", "max_time", max)
			return BatchDrop
		}
	}

	// We can do this check earlier, but it's a more intensive one, so we do this last.
	for i, txBytes := range batch.Transactions {
		if len(txBytes) == 0 {
			log.Warn("transaction data must not be empty, but found empty tx", "tx_index", i)
			return BatchDrop
		}
		if txBytes[0] == types.DepositTxType {
			log.Warn("sequencers may not embed any deposits into batch data, but found tx that has one", "tx_index", i)
			return BatchDrop
		}
	}

	return BatchAccept
}

// checkSpanBatchPrefix performs the span batch prefix rules for Holocene.
// Next to the validity, it also returns the parent L2 block as determined during the checks for
// further consumption.
func checkSpanBatchPrefix(ctx context.Context, cfg *rollup.Config, log log.Logger, l1Blocks []eth.L1BlockRef, l2SafeHead eth.L2BlockRef,
	batch *SpanBatch, l1InclusionBlock eth.L1BlockRef, l2Fetcher SafeBlockFetcher,
) (BatchValidity, eth.L2BlockRef) {
	// add details to the log
	log = batch.LogContext(log)

	// sanity check we have consistent inputs
	if len(l1Blocks) == 0 {
		log.Warn("missing L1 block input, cannot proceed with batch checking")
		return BatchUndecided, eth.L2BlockRef{}
	}
	epoch := l1Blocks[0]

	startEpochNum := uint64(batch.GetStartEpochNum())
	batchOrigin := epoch
	if startEpochNum == batchOrigin.Number+1 {
		if len(l1Blocks) < 2 {
			log.Info("eager batch wants to advance epoch, but could not without more L1 blocks", "current_epoch", epoch.ID())
			return BatchUndecided, eth.L2BlockRef{}
		}
		batchOrigin = l1Blocks[1]
	}
	if !cfg.IsDelta(batchOrigin.Time) {
		log.Warn("received SpanBatch with L1 origin before Delta hard fork", "l1_origin", batchOrigin.ID(), "l1_origin_time", batchOrigin.Time)
		return BatchDrop, eth.L2BlockRef{}
	}

	nextTimestamp := l2SafeHead.Time + cfg.BlockTime

	if batch.GetTimestamp() > nextTimestamp {
		if cfg.IsHolocene(l1InclusionBlock.Time) {
			log.Warn("dropping future span batch", "next_timestamp", nextTimestamp)
			return BatchDrop, eth.L2BlockRef{}
		}
		log.Trace("received out-of-order batch for future processing after next batch", "next_timestamp", nextTimestamp)
		return BatchFuture, eth.L2BlockRef{}
	}
	if batch.GetBlockTimestamp(batch.GetBlockCount()-1) < nextTimestamp {
		log.Warn("span batch has no new blocks after safe head")
		if cfg.IsHolocene(l1InclusionBlock.Time) {
			return BatchPast, eth.L2BlockRef{}
		}
		return BatchDrop, eth.L2BlockRef{}
	}

	// finding parent block of the span batch.
	// if the span batch does not overlap the current safe chain, parentBLock should be l2SafeHead.
	parentBlock := l2SafeHead
	if batch.GetTimestamp() < nextTimestamp {
		if batch.GetTimestamp() > l2SafeHead.Time {
			// batch timestamp cannot be between safe head and next timestamp
			log.Warn("batch has misaligned timestamp, block time is too short")
			return BatchDrop, eth.L2BlockRef{}
		}
		if (l2SafeHead.Time-batch.GetTimestamp())%cfg.BlockTime != 0 {
			log.Warn("batch has misaligned timestamp, not overlapped exactly")
			return BatchDrop, eth.L2BlockRef{}
		}
		parentNum := l2SafeHead.Number - (l2SafeHead.Time-batch.GetTimestamp())/cfg.BlockTime - 1
		var err error
		parentBlock, err = l2Fetcher.L2BlockRefByNumber(ctx, parentNum)
		if err != nil {
			log.Warn("failed to fetch L2 block", "number", parentNum, "err", err)
			// unable to validate the batch for now. retry later.
			return BatchUndecided, eth.L2BlockRef{}
		}
	}
	if !batch.CheckParentHash(parentBlock.Hash) {
		log.Warn("ignoring batch with mismatching parent hash", "parent_block", parentBlock.Hash)
		return BatchDrop, parentBlock
	}

	// Filter out batches that were included too late.
	if startEpochNum+cfg.SeqWindowSize < l1InclusionBlock.Number {
		log.Warn("batch was included too late, sequence window expired")
		return BatchDrop, parentBlock
	}

	// Check the L1 origin of the batch
	if startEpochNum > parentBlock.L1Origin.Number+1 {
		log.Warn("batch is for future epoch too far ahead, while it has the next timestamp, so it must be invalid", "current_epoch", epoch.ID())
		return BatchDrop, parentBlock
	}

	endEpochNum := batch.GetBlockEpochNum(batch.GetBlockCount() - 1)
	originChecked := false
	// l1Blocks is supplied from batch queue and its length is limited to SequencerWindowSize.
	for _, l1Block := range l1Blocks {
		if l1Block.Number == endEpochNum {
			if !batch.CheckOriginHash(l1Block.Hash) {
				log.Warn("batch is for different L1 chain, epoch hash does not match", "expected", l1Block.Hash)
				return BatchDrop, parentBlock
			}
			originChecked = true
			break
		}
	}
	if !originChecked {
		log.Info("need more l1 blocks to check entire origins of span batch")
		return BatchUndecided, parentBlock
	}

	if startEpochNum < parentBlock.L1Origin.Number {
		log.Warn("dropped batch, epoch is too old", "minimum", parentBlock.ID())
		return BatchDrop, parentBlock
	}
	return BatchAccept, parentBlock
}

// checkSpanBatch performs the full SpanBatch validation rules.
func checkSpanBatch(ctx context.Context, cfg *rollup.Config, log log.Logger, l1Blocks []eth.L1BlockRef, l2SafeHead eth.L2BlockRef,
	batch *SpanBatch, l1InclusionBlock eth.L1BlockRef, l2Fetcher SafeBlockFetcher,
) BatchValidity {
	prefixValidity, parentBlock := checkSpanBatchPrefix(ctx, cfg, log, l1Blocks, l2SafeHead, batch, l1InclusionBlock, l2Fetcher)
	if prefixValidity != BatchAccept {
		return prefixValidity
	}

	startEpochNum := uint64(batch.GetStartEpochNum())

	originIdx := 0
	originAdvanced := startEpochNum == parentBlock.L1Origin.Number+1

	for i := 0; i < batch.GetBlockCount(); i++ {
		if batch.GetBlockTimestamp(i) <= l2SafeHead.Time {
			continue
		}
		var l1Origin eth.L1BlockRef
		for j := originIdx; j < len(l1Blocks); j++ {
			if batch.GetBlockEpochNum(i) == l1Blocks[j].Number {
				l1Origin = l1Blocks[j]
				originIdx = j
				break
			}
		}
		if i > 0 {
			originAdvanced = false
			if batch.GetBlockEpochNum(i) > batch.GetBlockEpochNum(i-1) {
				originAdvanced = true
			}
		}
		blockTimestamp := batch.GetBlockTimestamp(i)
		if blockTimestamp < l1Origin.Time {
			log.Warn("block timestamp is less than L1 origin timestamp", "l2_timestamp", blockTimestamp, "l1_timestamp", l1Origin.Time, "origin", l1Origin.ID())
			return BatchDrop
		}

		spec := rollup.NewChainSpec(cfg)
		// Check if we ran out of sequencer time drift
		if max := l1Origin.Time + spec.MaxSequencerDrift(l1Origin.Time); blockTimestamp > max {
			if len(batch.GetBlockTransactions(i)) == 0 {
				// If the sequencer is co-operating by producing an empty batch,
				// then allow the batch if it was the right thing to do to maintain the L2 time >= L1 time invariant.
				// We only check batches that do not advance the epoch, to ensure epoch advancement regardless of time drift is allowed.
				if !originAdvanced {
					if originIdx+1 >= len(l1Blocks) {
						log.Info("without the next L1 origin we cannot determine yet if this empty batch that exceeds the time drift is still valid")
						return BatchUndecided
					}
					if blockTimestamp >= l1Blocks[originIdx+1].Time { // check if the next L1 origin could have been adopted
						log.Info("batch exceeded sequencer time drift without adopting next origin, and next L1 origin would have been valid")
						return BatchDrop
					} else {
						log.Info("continuing with empty batch before late L1 block to preserve L2 time invariant")
					}
				}
			} else {
				// If the sequencer is ignoring the time drift rule, then drop the batch and force an empty batch instead,
				// as the sequencer is not allowed to include anything past this point without moving to the next epoch.
				log.Warn("batch exceeded sequencer time drift, sequencer must adopt new L1 origin to include transactions again", "max_time", max)
				return BatchDrop
			}
		}

		for i, txBytes := range batch.GetBlockTransactions(i) {
			if len(txBytes) == 0 {
				log.Warn("transaction data must not be empty, but found empty tx", "tx_index", i)
				return BatchDrop
			}
			if txBytes[0] == types.DepositTxType {
				log.Warn("sequencers may not embed any deposits into batch data, but found tx that has one", "tx_index", i)
				return BatchDrop
			}
		}
	}

	parentNum := parentBlock.Number
	nextTimestamp := l2SafeHead.Time + cfg.BlockTime
	// Check overlapped blocks
	if batch.GetTimestamp() < nextTimestamp {
		for i := uint64(0); i < l2SafeHead.Number-parentNum; i++ {
			safeBlockNum := parentNum + i + 1
			safeBlockPayload, err := l2Fetcher.PayloadByNumber(ctx, safeBlockNum)
			if err != nil {
				log.Warn("failed to fetch L2 block payload", "number", parentNum, "err", err)
				// unable to validate the batch for now. retry later.
				return BatchUndecided
			}
			safeBlockTxs := safeBlockPayload.ExecutionPayload.Transactions
			batchTxs := batch.GetBlockTransactions(int(i))
			// execution payload has deposit TXs, but batch does not.
			depositCount := 0
			for _, tx := range safeBlockTxs {
				if tx[0] == types.DepositTxType {
					depositCount++
				}
			}
			if len(safeBlockTxs)-depositCount != len(batchTxs) {
				log.Warn("overlapped block's tx count does not match", "safeBlockTxs", len(safeBlockTxs), "batchTxs", len(batchTxs))
				return BatchDrop
			}
			for j := 0; j < len(batchTxs); j++ {
				if !bytes.Equal(safeBlockTxs[j+depositCount], batchTxs[j]) {
					log.Warn("overlapped block's transaction does not match")
					return BatchDrop
				}
			}
			safeBlockRef, err := PayloadToBlockRef(cfg, safeBlockPayload.ExecutionPayload)
			if err != nil {
				log.Error("failed to extract L2BlockRef from execution payload", "hash", safeBlockPayload.ExecutionPayload.BlockHash, "err", err)
				return BatchDrop
			}
			if safeBlockRef.L1Origin.Number != batch.GetBlockEpochNum(int(i)) {
				log.Warn("overlapped block's L1 origin number does not match")
				return BatchDrop
			}
		}
	}

	return BatchAccept
}
