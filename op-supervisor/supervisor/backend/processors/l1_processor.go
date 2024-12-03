package processors

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum-optimism/optimism/op-service/client"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/sources"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
	"github.com/ethereum/go-ethereum/log"
)

type chainsDB interface {
	RecordNewL1(ref eth.BlockRef) error
	LastCommonL1() (types.BlockSeal, error)
}

type l1Client interface {
	L1BlockRefByNumber(ctx context.Context, number uint64) (eth.L1BlockRef, error)
}

type L1Processor struct {
	log    log.Logger
	client l1Client

	currentNumber uint64
	tickDuration  time.Duration

	db chainsDB

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewL1Processor(log log.Logger, cdb chainsDB, l1RPCAddr string) (*L1Processor, error) {
	ctx, cancel := context.WithCancel(context.Background())
	l1RPC, err := client.NewRPC(ctx, log, l1RPCAddr)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to setup L1 RPC: %w", err)
	}
	l1Client, err := sources.NewL1Client(
		l1RPC,
		log,
		nil,
		// placeholder config for the L1
		sources.L1ClientSimpleConfig(true, sources.RPCKindBasic, 100))
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to setup L1 Client: %w", err)
	}
	return &L1Processor{
		client:       l1Client,
		db:           cdb,
		log:          log.New("service", "l1-processor"),
		tickDuration: 6 * time.Second,
		ctx:          ctx,
		cancel:       cancel,
	}, nil
}

func (p *L1Processor) Start() {
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
	p.cancel()
	p.wg.Wait()
}

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

func (p *L1Processor) work() error {
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
