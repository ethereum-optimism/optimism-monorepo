package l1access

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-node/rollup/event"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/superevents"
)

const reqTimeout = time.Second * 10

type L1Source interface {
	L1BlockRefByNumber(ctx context.Context, number uint64) (eth.L1BlockRef, error)
	L1BlockRefByLabel(ctx context.Context, label eth.BlockLabel) (eth.L1BlockRef, error)
}

// L1Accessor provides access to the L1 chain.
// it wraps an L1 source in order to pass calls to the L1 chain
// and manages the finality and latest block subscriptions.
// The finality subscription is hooked to a finality handler function provided by the caller.
// and the latest block subscription is used to monitor the tip height of the L1 chain.
// L1Accessor has the concept of confirmation depth, which is used to block access to requests to blocks which are too recent.
// When requests for blocks are more recent than the tip minus the confirmation depth, a NotFound error is returned.
type L1Accessor struct {
	log log.Logger

	client   L1Source // may be nil if no source is attached
	clientMu sync.RWMutex

	emitter event.Emitter

	finalitySub ethereum.Subscription

	// tipHeight is the height of the L1 chain tip
	// used to block access to requests more recent than the confirmation depth
	tipHeight uint64
	latestSub ethereum.Subscription
	confDepth uint64

	// to interrupt requests, so the system can shut down quickly
	sysCtx context.Context
}

var _ event.AttachEmitter = (*L1Accessor)(nil)

func NewL1Accessor(sysCtx context.Context, log log.Logger, client L1Source) *L1Accessor {
	return &L1Accessor{
		log:    log.New("service", "l1-processor"),
		client: client,
		// placeholder confirmation depth
		confDepth: 2,
		sysCtx:    sysCtx,
	}
}

func (p *L1Accessor) AttachEmitter(em event.Emitter) {
	p.emitter = em
}

func (p *L1Accessor) OnEvent(ev event.Event) bool {
	return false
}

// AttachClient attaches a new client to the processor.
// If an existing client was attached, the old subscriptions are unsubscribed.
// New subscriptions are created if subscribe is true.
// If subscribe is false, L1 status has to be fetched manually with PullFinalized and PullLatest.
func (p *L1Accessor) AttachClient(client L1Source, subscribe bool) {
	p.clientMu.Lock()
	defer p.clientMu.Unlock()

	// if we have a finality subscription, unsubscribe from it
	if p.finalitySub != nil {
		p.finalitySub.Unsubscribe()
	}

	// if we have a latest subscription, unsubscribe from it
	if p.latestSub != nil {
		p.latestSub.Unsubscribe()
	}

	p.client = client

	if client != nil && subscribe {
		p.SubscribeLatestHandler()
		p.SubscribeFinalityHandler()
	}
}

func (p *L1Accessor) SubscribeFinalityHandler() {
	p.finalitySub = eth.PollBlockChanges(
		p.log,
		p.client,
		p.onFinalized,
		eth.Finalized,
		3*time.Second,
		reqTimeout)
}

func (p *L1Accessor) SubscribeLatestHandler() {
	p.latestSub = eth.PollBlockChanges(
		p.log,
		p.client,
		p.onLatest,
		eth.Unsafe,
		3*time.Second,
		reqTimeout)
}

func (p *L1Accessor) SetConfDepth(depth uint64) {
	p.confDepth = depth
}

func (p *L1Accessor) PullFinalized() error {
	p.clientMu.RLock()
	defer p.clientMu.RUnlock()
	if p.client == nil {
		return errors.New("no L1 source configured")
	}

	ctx, cancel := context.WithTimeout(p.sysCtx, reqTimeout)
	defer cancel()
	ref, err := p.client.L1BlockRefByLabel(ctx, eth.Finalized)
	if err != nil {
		return fmt.Errorf("failed to pull finalized block ref: %w", err)
	}
	p.onFinalized(p.sysCtx, ref)
	return nil
}

func (p *L1Accessor) PullLatest() error {
	p.clientMu.RLock()
	defer p.clientMu.RUnlock()

	if p.client == nil {
		return errors.New("no L1 source configured")
	}

	ctx, cancel := context.WithTimeout(p.sysCtx, reqTimeout)
	defer cancel()
	ref, err := p.client.L1BlockRefByLabel(ctx, eth.Unsafe)
	if err != nil {
		return fmt.Errorf("failed to pull latest block ref: %w", err)
	}
	p.onLatest(p.sysCtx, ref)
	return nil
}

func (p *L1Accessor) onFinalized(ctx context.Context, ref eth.L1BlockRef) {
	p.emitter.Emit(superevents.FinalizedL1RequestEvent{FinalizedL1: ref})
}

func (p *L1Accessor) onLatest(ctx context.Context, ref eth.L1BlockRef) {
	p.tipHeight = ref.Number
}

func (p *L1Accessor) L1BlockRefByNumber(ctx context.Context, number uint64) (eth.L1BlockRef, error) {
	p.clientMu.RLock()
	defer p.clientMu.RUnlock()
	if p.client == nil {
		return eth.L1BlockRef{}, errors.New("no L1 source available")
	}
	// block access to requests more recent than the confirmation depth
	if number > p.tipHeight-p.confDepth {
		return eth.L1BlockRef{}, ethereum.NotFound
	}
	return p.client.L1BlockRefByNumber(ctx, number)
}
