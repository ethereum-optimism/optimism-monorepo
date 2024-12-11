package processors

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
	"github.com/ethereum/go-ethereum/log"
)

type chainsDB interface {
	RecordNewL1(ref eth.BlockRef) error
	LastCommonL1() (types.BlockSeal, error)
}

type L1Source interface {
	L1BlockRefByNumber(ctx context.Context, number uint64) (eth.L1BlockRef, error)
}

type L1Processor struct {
	log      log.Logger
	client   L1Source
	clientMu sync.Mutex
	running  atomic.Bool

	currentNumber uint64
	tickDuration  time.Duration

	db chainsDB

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewL1Processor(log log.Logger, cdb chainsDB, client L1Source) *L1Processor {
	ctx, cancel := context.WithCancel(context.Background())
	return &L1Processor{
		client:       client,
		db:           cdb,
		log:          log.New("service", "l1-processor"),
		tickDuration: 6 * time.Second,
		ctx:          ctx,
		cancel:       cancel,
	}
}

func (p *L1Processor) AttachClient(client L1Source) {
	p.clientMu.Lock()
	defer p.clientMu.Unlock()
	p.client = client
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
	p.clientMu.Lock()
	defer p.clientMu.Unlock()
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

	// go drive derivation on this new L1 input
	// only possible once bidirectional RPC and new derivers are in place
	// could do this as a function callback to a more appropriate driver

	// update the target number
	p.currentNumber = nextNumber
	return nil
}
