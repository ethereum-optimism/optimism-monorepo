package processors

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-node/rollup/event"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/superevents"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

type Source interface {
	BlockRefByNumber(ctx context.Context, number uint64) (eth.BlockRef, error)
	FetchReceipts(ctx context.Context, blockHash common.Hash) (gethtypes.Receipts, error)
}

type LogProcessor interface {
	ProcessLogs(ctx context.Context, block eth.BlockRef, receipts gethtypes.Receipts) error
}

type DatabaseRewinder interface {
	Rewind(chain eth.ChainID, headBlock eth.BlockID) error
	LatestBlockNum(chain eth.ChainID) (num uint64, ok bool)
	AcceptedBlock(chainID eth.ChainID, id eth.BlockID) error
}

type BlockProcessorFn func(ctx context.Context, block eth.BlockRef) error

func (fn BlockProcessorFn) ProcessBlock(ctx context.Context, block eth.BlockRef) error {
	return fn(ctx, block)
}

// ChainProcessor is a HeadProcessor that fills in any skipped blocks between head update events.
// It ensures that, absent reorgs, every block in the chain is processed even if some head advancements are skipped.
type ChainProcessor struct {
	log log.Logger

	client     Source
	clientLock sync.Mutex

	chain eth.ChainID

	systemContext context.Context

	processor LogProcessor
	rewinder  DatabaseRewinder

	emitter event.Emitter

	maxFetcherThreads int
}

var _ event.AttachEmitter = (*ChainProcessor)(nil)
var _ event.Deriver = (*ChainProcessor)(nil)

func NewChainProcessor(systemContext context.Context, log log.Logger, chain eth.ChainID, processor LogProcessor, rewinder DatabaseRewinder) *ChainProcessor {
	out := &ChainProcessor{
		systemContext:     systemContext,
		log:               log.New("chain", chain),
		client:            nil,
		chain:             chain,
		processor:         processor,
		rewinder:          rewinder,
		maxFetcherThreads: 10,
	}
	return out
}

func (s *ChainProcessor) AttachEmitter(em event.Emitter) {
	s.emitter = em
}

func (s *ChainProcessor) SetSource(cl Source) {
	s.clientLock.Lock()
	defer s.clientLock.Unlock()
	s.client = cl
}

func (s *ChainProcessor) nextNum() uint64 {
	headNum, ok := s.rewinder.LatestBlockNum(s.chain)
	if !ok {
		return 0 // genesis. We could change this to start at a later block.
	}
	return headNum + 1
}

func (s *ChainProcessor) OnEvent(ev event.Event) bool {
	switch x := ev.(type) {
	case superevents.ChainProcessEvent:
		if x.ChainID != s.chain {
			return false
		}
		s.onRequest(x.Target)
	default:
		return false
	}
	return true
}

func (s *ChainProcessor) onRequest(target uint64) {
	_, err := s.rangeUpdate(target)
	if err != nil {
		if errors.Is(err, ethereum.NotFound) {
			s.log.Debug("Event-indexer cannot find next block yet", "target", target, "err", err)
		} else if errors.Is(err, types.ErrNoRPCSource) {
			s.log.Warn("No RPC source configured, cannot process new blocks")
		} else {
			s.log.Error("Failed to process new block", "err", err)
			s.emitter.Emit(superevents.RewindChainEvent{
				ChainID:        s.chain,
				BadBlockHeight: target,
			})
		}
	} else if x := s.nextNum(); x <= target {
		s.log.Debug("Continuing with next block", "target", target, "next", x)
		s.emitter.Emit(superevents.ChainProcessEvent{
			ChainID: s.chain,
			Target:  target,
		}) // instantly continue processing, no need to idle
	} else {
		s.log.Debug("Idling block-processing, reached latest block", "head", target)
	}
}

func (s *ChainProcessor) rangeUpdate(target uint64) (int, error) {
	s.clientLock.Lock()
	defer s.clientLock.Unlock()
	if s.client == nil {
		return 0, types.ErrNoRPCSource
	}

	// define the range of blocks to fetch
	// [next, last] inclusive with a max of s.fetcherThreads blocks
	next := s.nextNum()
	last := target

	nums := make([]uint64, 0, s.maxFetcherThreads)
	for i := next; i <= last; i++ {
		nums = append(nums, i)
		// only attempt as many blocks as we can fetch in parallel
		if len(nums) >= s.maxFetcherThreads {
			s.log.Debug("Fetching up to max threads", "chain", s.chain.String(), "next", next, "last", last, "count", len(nums))
			break
		}
	}

	if len(nums) == 0 {
		s.log.Debug("No blocks to fetch", "chain", s.chain.String(), "next", next, "last", last)
		return 0, nil
	}

	s.log.Debug("Fetching blocks", "chain", s.chain.String(), "next", next, "last", last, "count", len(nums))

	// make a structure to receive parallel results
	type keyedResult struct {
		num      uint64
		blockRef *eth.BlockRef
		receipts gethtypes.Receipts
		err      error
	}
	parallelResults := make(chan keyedResult, len(nums))

	// each thread will fetch a block and its receipts and send the result to the channel
	fetch := func(wg *sync.WaitGroup, num uint64) {
		defer wg.Done()
		// ensure we emit the result at the end
		result := keyedResult{num, nil, nil, nil}
		defer func() { parallelResults <- result }()

		// fetch the block ref
		ctx, cancel := context.WithTimeout(s.systemContext, time.Second*10)
		next, err := s.client.BlockRefByNumber(ctx, num)
		cancel()
		if err != nil {
			result.err = err
			return
		}
		if err := s.rewinder.AcceptedBlock(s.chain, next.ID()); err != nil {
			s.log.Warn("Cannot accept next block into events DB", "err", err)
			result.err = err
			return
		}
		result.blockRef = &next

		// fetch receipts
		ctx, cancel = context.WithTimeout(s.systemContext, time.Second*10)
		receipts, err := s.client.FetchReceipts(ctx, next.Hash)
		cancel()
		if err != nil {
			result.err = err
			return
		}
		result.receipts = receipts
	}

	// kick off the fetches and wait for them to complete
	var wg sync.WaitGroup
	for _, num := range nums {
		wg.Add(1)
		go fetch(&wg, num)
	}
	wg.Wait()

	// collect and sort the results
	results := make([]keyedResult, len(nums))
	for i := range nums {
		result := <-parallelResults
		results[i] = result
	}
	slices.SortFunc(results, func(a, b keyedResult) int {
		if a.num < b.num {
			return -1
		}
		if a.num > b.num {
			return 1
		}
		return 0
	})

	// process the results in order and return the first error encountered,
	// and the number of blocks processed successfully by this call
	for i := range results {
		if results[i].err != nil {
			return i, fmt.Errorf("failed to fetch block %d: %w", results[i].num, results[i].err)
		}
		// process the receipts
		err := s.process(s.systemContext, *results[i].blockRef, results[i].receipts)
		if err != nil {
			return i, fmt.Errorf("failed to process block %d: %w", results[i].num, err)
		}
	}
	return len(results), nil
}

func (s *ChainProcessor) process(ctx context.Context, next eth.BlockRef, receipts gethtypes.Receipts) error {
	if err := s.processor.ProcessLogs(ctx, next, receipts); err != nil {
		s.log.Error("Failed to process block", "block", next, "err", err)

		if next.Number == 0 { // cannot rewind genesis
			return nil
		}

		// Try to rewind the database to the previous block to remove any logs from this block that were written
		if err := s.rewinder.Rewind(s.chain, next.ParentID()); err != nil {
			// If any logs were written, our next attempt to write will fail and we'll retry this rewind.
			// If no logs were written successfully then the rewind wouldn't have done anything anyway.
			s.log.Error("Failed to rewind after error processing block", "block", next, "err", err)
		}
		return err
	}
	s.log.Info("Indexed block events", "block", next, "txs", len(receipts))
	return nil
}

// ResetToBlock resets the chain processor to the given block
func (s *ChainProcessor) ResetToBlock(ctx context.Context, ref eth.L2BlockRef) error {
	s.log.Info("Resetting chain processor", "ref", ref)

	// Rewind the database to the given block
	if err := s.rewinder.Rewind(s.chain, ref.ID()); err != nil {
		return fmt.Errorf("failed to rewind database: %w", err)
	}

	// Emit a chain process event to restart processing from the new block
	s.emitter.Emit(superevents.ChainProcessEvent{
		ChainID: s.chain,
		Target:  ref.Number,
	})

	return nil
}
