package processors

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/log"
)

type chainsDB interface {
	RecordNewL1(ref eth.BlockRef) error
	LastCommonL1() (types.BlockSeal, error)
	FinalizedL1() eth.BlockRef
	UpdateFinalizedL1(finalized eth.BlockRef) error
}

type controller interface {
	DeriveFromL1(eth.BlockRef) error
}

type L1Source interface {
	L1BlockRefByNumber(ctx context.Context, number uint64) (eth.L1BlockRef, error)
	L1BlockRefByLabel(ctx context.Context, label eth.BlockLabel) (eth.L1BlockRef, error)
}

type L1Processor struct {
	log         log.Logger
	client      L1Source
	clientMu    sync.RWMutex
	running     atomic.Bool
	finalitySub ethereum.Subscription

	currentNumber uint64
	tickDuration  time.Duration

	db  chainsDB
	snc controller

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewL1Processor(log log.Logger, cdb chainsDB, snc controller, client L1Source) *L1Processor {
	ctx, cancel := context.WithCancel(context.Background())
	tickDuration := 6 * time.Second
	return &L1Processor{
		client:       client,
		db:           cdb,
		snc:          snc,
		log:          log.New("service", "l1-processor"),
		tickDuration: tickDuration,
		ctx:          ctx,
		cancel:       cancel,
	}
}

func (p *L1Processor) AttachClient(client L1Source) {
	p.clientMu.Lock()
	defer p.clientMu.Unlock()
	// unsubscribe from the old client
	if p.finalitySub != nil {
		p.finalitySub.Unsubscribe()
	}
	// make the new client the active one
	p.client = client
	// resubscribe to the new client
	p.finalitySub = eth.PollBlockChanges(
		p.log,
		p.client,
		p.handleFinalized,
		eth.Finalized,
		3*time.Second,
		10*time.Second)
}

func (p *L1Processor) Start() {
	// if already running, do nothing
	if p.running.Load() {
		return
	}
	p.running.Store(true)
	p.currentNumber = 0
	// if there is an issue getting the last common L1, default to starting from 0
	// consider making this a fatal error in the future once initialization is more robust
	if lastL1, err := p.db.LastCommonL1(); err == nil {
		p.currentNumber = lastL1.Number
	}
	p.wg.Add(1)
	go p.worker()
	p.finalitySub = eth.PollBlockChanges(
		p.log,
		p.client,
		p.handleFinalized,
		eth.Finalized,
		p.tickDuration,
		p.tickDuration)
}

func (p *L1Processor) Stop() {
	// if not running, do nothing
	if !p.running.Load() {
		return
	}
	p.cancel()
	p.wg.Wait()
	p.running.Store(false)
}

// worker runs a loop that checks for new L1 blocks at a regular interval
func (p *L1Processor) worker() {
	defer p.wg.Done()
	delay := time.NewTicker(p.tickDuration)
	for {
		select {
		case <-p.ctx.Done():
			return
		case <-delay.C:
			p.log.Debug("Checking for new L1 block", "current", p.currentNumber)
			err := p.work()
			if err != nil {
				p.log.Warn("Failed to process L1", "err", err)
			}
		}
	}
}

// work checks for a new L1 block and processes it if found
// the starting point is set when Start is called, and blocks are processed searched incrementally
// if a new block is found, it is recorded in the database and the target number is updated
// in the future it will also kick of derivation management for the sync nodes
func (p *L1Processor) work() error {
	p.clientMu.RLock()
	defer p.clientMu.RUnlock()
	nextNumber := p.currentNumber + 1
	ref, err := p.client.L1BlockRefByNumber(p.ctx, nextNumber)
	if err != nil {
		return err
	}
	// record the new L1 block
	p.log.Debug("Processing new L1 block", "block", ref)
	err = p.db.RecordNewL1(ref)
	if err != nil {
		return err
	}

	// send the new L1 block to the sync nodes for derivation
	if err := p.snc.DeriveFromL1(ref); err != nil {
		return err
	}

	// update the target number
	p.currentNumber = nextNumber
	return nil
}

// handleFinalized is called when a new finalized block is received from the L1 chain subscription
// it updates the database with the new finalized block if it is newer than the current one
func (p *L1Processor) handleFinalized(ctx context.Context, sig eth.L1BlockRef) {
	MaybeUpdateFinalizedL1(ctx, p.log, p.db, sig)
}

// MaybeUpdateFinalizedL1 updates the database with the new finalized block if it is newer than the current one
// it is defined outside of the L1Processor so tests can call it directly without having a processor
func MaybeUpdateFinalizedL1(ctx context.Context, logger log.Logger, db chainsDB, ref eth.L1BlockRef) {
	// do something with the new block
	logger.Debug("Received new Finalized L1 block", "block", ref)
	currentFinalized := db.FinalizedL1()
	if currentFinalized.Number > ref.Number {
		logger.Warn("Finalized block in database is newer than subscribed finalized block", "current", currentFinalized, "new", ref)
		return
	}
	if ref.Number > currentFinalized.Number || currentFinalized == (eth.BlockRef{}) {
		// update the database with the new finalized block
		if err := db.UpdateFinalizedL1(ref); err != nil {
			logger.Warn("Failed to update finalized L1", "err", err)
			return
		}
		logger.Debug("Updated finalized L1 block", "block", ref)
	}

}
