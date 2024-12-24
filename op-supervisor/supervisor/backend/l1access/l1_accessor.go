package l1access

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/log"
)

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
	log      log.Logger
	client   L1Source // may be nil if no source is attached
	clientMu sync.RWMutex

	finalityHandler eth.HeadSignalFn
	finalitySub     ethereum.Subscription

	// tipHeight is the height of the L1 chain tip
	// used to block access to requests more recent than the confirmation depth
	tipHeight uint64
	latestSub ethereum.Subscription
	confDepth uint64
}

func NewL1Accessor(log log.Logger, client L1Source, finalityHandler eth.HeadSignalFn) *L1Accessor {
	return &L1Accessor{
		log:             log.New("service", "l1-processor"),
		client:          client,
		finalityHandler: finalityHandler,
		// placeholder confirmation depth
		confDepth: 2,
	}
}

// AttachClient attaches a new client to the processor
// if an existing client is attached, the old subscriptions are unsubscribed
// and new subscriptions are created
func (p *L1Accessor) AttachClient(client L1Source) {
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

	// resubscribe to the finality handler
	p.SubscribeFinalityHandler()

	// if we have a handler function, resubscribe to the finality handler
	if p.finalityHandler != nil {
		p.SubscribeFinalityHandler()
	}
}

func (p *L1Accessor) SubscribeFinalityHandler() {
	p.finalitySub = eth.PollBlockChanges(
		p.log,
		p.client,
		p.finalityHandler,
		eth.Finalized,
		3*time.Second,
		10*time.Second)
}

func (p *L1Accessor) SubscribeLatestHandler() {
	p.latestSub = eth.PollBlockChanges(
		p.log,
		p.client,
		p.SetTipHeight,
		eth.Unsafe,
		3*time.Second,
		10*time.Second)
}

func (p *L1Accessor) SetTipHeight(ctx context.Context, ref eth.L1BlockRef) {
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
