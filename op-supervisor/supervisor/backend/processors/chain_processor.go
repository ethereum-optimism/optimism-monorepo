package processors

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

type Source interface {
	L1BlockRefByNumber(ctx context.Context, number uint64) (eth.L1BlockRef, error)
	FetchReceipts(ctx context.Context, blockHash common.Hash) (eth.BlockInfo, gethtypes.Receipts, error)
}

type LogProcessor interface {
	ProcessLogs(ctx context.Context, block eth.BlockRef, receipts gethtypes.Receipts) error
}

type DatabaseRewinder interface {
	Rewind(chain types.ChainID, headBlockNum uint64) error
	LatestBlockNum(chain types.ChainID) (num uint64, ok bool)
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

	chain types.ChainID

	processor LogProcessor
	rewinder  DatabaseRewinder

	// the last known head. May be 0 if not known.
	lastHead atomic.Uint64
	// channel with capacity of 1, full if there is work to do
	newHead chan struct{}

	// to signal to the other services that new indexed data is available
	onIndexed func()

	// lifetime management of the chain processor
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewChainProcessor(log log.Logger, chain types.ChainID, processor LogProcessor, rewinder DatabaseRewinder, onIndexed func()) *ChainProcessor {
	ctx, cancel := context.WithCancel(context.Background())
	out := &ChainProcessor{
		log:       log.New("chain", chain),
		client:    nil,
		chain:     chain,
		processor: processor,
		rewinder:  rewinder,
		newHead:   make(chan struct{}, 1),
		onIndexed: onIndexed,
		ctx:       ctx,
		cancel:    cancel,
	}
	return out
}

func (s *ChainProcessor) SetSource(cl Source) {
	s.clientLock.Lock()
	defer s.clientLock.Unlock()
	s.client = cl
}

func (s *ChainProcessor) StartBackground() {
	s.wg.Add(1)
	go s.worker()
}

func (s *ChainProcessor) ProcessToHead() {
	s.work()
}

func (s *ChainProcessor) nextNum() uint64 {
	headNum, ok := s.rewinder.LatestBlockNum(s.chain)
	if !ok {
		return 0 // genesis. We could change this to start at a later block.
	}
	return headNum + 1
}

// worker is the main loop of the chain processor's worker
// it manages work by request or on a timer, and watches for shutdown
func (s *ChainProcessor) worker() {
	defer s.wg.Done()

	delay := time.NewTicker(time.Second * 5)
	for {
		// await next time we process, or detect shutdown
		select {
		case <-s.ctx.Done():
			delay.Stop()
			return
		case <-s.newHead:
			s.log.Debug("Responding to new head signal")
			s.work()
		case <-delay.C:
			s.log.Debug("Checking for updates")
			s.work()
		}
	}
}

// work processes the next block in the chain repeatedly until it reaches the head
func (s *ChainProcessor) work() {
	for {
		if s.ctx.Err() != nil { // check if we are closing down
			return
		}
		target := s.nextNum()
		if err := s.update(target); err != nil {
			if errors.Is(err, ethereum.NotFound) {
				s.log.Debug("Event-indexer cannot find next block yet", "target", target, "err", err)
			} else if errors.Is(err, types.ErrNoRPCSource) {
				s.log.Warn("No RPC source configured, cannot process new blocks")
			} else {
				s.log.Error("Failed to process new block", "err", err)
				// idle until next update trigger
			}
		} else if x := s.lastHead.Load(); target+1 <= x {
			s.log.Debug("Continuing with next block", "newTarget", target+1, "lastHead", x)
			continue // instantly continue processing, no need to idle
		} else {
			s.log.Debug("Idling block-processing, reached latest block", "head", target)
		}
		return
	}
}

func (s *ChainProcessor) update(nextNum uint64) error {
	s.clientLock.Lock()
	defer s.clientLock.Unlock()

	if s.client == nil {
		return types.ErrNoRPCSource
	}

	ctx, cancel := context.WithTimeout(s.ctx, time.Second*10)
	nextL1, err := s.client.L1BlockRefByNumber(ctx, nextNum)
	next := eth.BlockRef{
		Hash:       nextL1.Hash,
		ParentHash: nextL1.ParentHash,
		Number:     nextL1.Number,
		Time:       nextL1.Time,
	}
	cancel()
	if err != nil {
		return fmt.Errorf("failed to fetch next block: %w", err)
	}

	// Try and fetch the receipts
	ctx, cancel = context.WithTimeout(s.ctx, time.Second*10)
	_, receipts, err := s.client.FetchReceipts(ctx, next.Hash)
	cancel()
	if err != nil {
		return fmt.Errorf("failed to fetch receipts of block: %w", err)
	}
	if err := s.processor.ProcessLogs(ctx, next, receipts); err != nil {
		s.log.Error("Failed to process block", "block", next, "err", err)

		if next.Number == 0 { // cannot rewind genesis
			return nil
		}

		// Try to rewind the database to the previous block to remove any logs from this block that were written
		if err := s.rewinder.Rewind(s.chain, nextNum-1); err != nil {
			// If any logs were written, our next attempt to write will fail and we'll retry this rewind.
			// If no logs were written successfully then the rewind wouldn't have done anything anyway.
			s.log.Error("Failed to rewind after error processing block", "block", next, "err", err)
		}
		return err
	}
	s.log.Info("Indexed block events", "block", next, "txs", len(receipts))
	s.onIndexed()
	return nil
}

func (s *ChainProcessor) OnNewHead(head eth.BlockRef) error {
	// update the latest target
	s.lastHead.Store(head.Number)
	// signal that we have something to process
	select {
	case s.newHead <- struct{}{}:
	default:
		// already requested an update
	}
	return nil
}

func (s *ChainProcessor) Close() {
	s.cancel()
	s.wg.Wait()
}
