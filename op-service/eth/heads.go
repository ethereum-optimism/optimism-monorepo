package eth

import (
	"context"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
)

// HeadSignalFn is used as a callback function to accept head-signals
type HeadSignalFn func(ctx context.Context, sig L1BlockRef)

type NewHeadSource interface {
	SubscribeNewHead(ctx context.Context, ch chan<- *types.Header) (ethereum.Subscription, error)
}

// WatchHeadChanges wraps a new-head subscription from NewHeadSource to feed the given Tracker.
// The ctx is only used to create the subscription, and does not affect the returned subscription.
func WatchHeadChanges(ctx context.Context, src NewHeadSource, fn HeadSignalFn) (ethereum.Subscription, error) {
	headChanges := make(chan *types.Header, 10)
	sub, err := src.SubscribeNewHead(ctx, headChanges)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		eventsCtx, eventsCancel := context.WithCancel(context.Background())
		defer sub.Unsubscribe()
		defer eventsCancel()

		// We can handle a quit signal while fn is running, by closing the ctx.
		go func() {
			select {
			case <-quit:
				eventsCancel()
			case <-eventsCtx.Done(): // don't wait for quit signal if we closed for other reasons.
				return
			}
		}()

		for {
			select {
			case header := <-headChanges:
				fn(eventsCtx, L1BlockRef{
					Hash:       header.Hash(),
					Number:     header.Number.Uint64(),
					ParentHash: header.ParentHash,
					Time:       header.Time,
				})
			case <-eventsCtx.Done():
				return nil
			case err := <-sub.Err(): // if the underlying subscription fails, stop
				return err
			}
		}
	}), nil
}

type L1BlockRefsSource interface {
	L1BlockRefByLabel(ctx context.Context, label BlockLabel) (L1BlockRef, error)
}

// PollBlockChanges opens a polling loop to fetch the L1 block reference with the given label,
// on provided interval and with request timeout. Results are returned with provided callback fn,
// which may block to pause/back-pressure polling.
func PollBlockChanges(log log.Logger, src L1BlockRefsSource, fn HeadSignalFn,
	label BlockLabel, interval time.Duration, timeout time.Duration) ethereum.Subscription {
	return event.NewSubscription(func(quit <-chan struct{}) error {
		if interval <= 0 {
			log.Warn("polling of block is disabled", "interval", interval, "label", label)
			<-quit
			return nil
		}
		eventsCtx, eventsCancel := context.WithCancel(context.Background())
		defer eventsCancel()
		// We can handle a quit signal while fn is running, by closing the ctx.
		go func() {
			select {
			case <-quit:
				eventsCancel()
			case <-eventsCtx.Done(): // don't wait for quit signal if we closed for other reasons.
				return
			}
		}()

		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				reqCtx, reqCancel := context.WithTimeout(eventsCtx, timeout)
				ref, err := src.L1BlockRefByLabel(reqCtx, label)
				reqCancel()
				if err != nil {
					log.Warn("failed to poll L1 block", "label", label, "err", err)
				} else {
					fn(eventsCtx, ref)
				}
			case <-eventsCtx.Done():
				return nil
			}
		}
	})
}
