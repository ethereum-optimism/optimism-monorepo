package rpc

import (
	"context"
	"errors"
	"slices"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	gethrpc "github.com/ethereum/go-ethereum/rpc"
)

// OutOfEventsErrCode is the RPC error-code used to signal that no buffered events are available to dequeue.
// A polling RPC client should back off in this case.
const OutOfEventsErrCode = -39001

// EventEntry wraps subscription data, so the server can communicate alternative metadata,
// such as close instructions.
type EventEntry[E any] struct {
	// Data wraps the actual event object. It may be nil if Close is true.
	Data *E `json:"data"`
	// Close is set to true when the server will send no further events over this subscription.
	Close bool `json:"close,omitempty"`
}

// StreamFallback polls the given function for data.
// When the function returns a JSON RPC error with OutOfEventsErrCode error-code,
// the polling backs off by waiting for the given frequency time duration.
// When the function returns any other error, the stream is aborted,
// and the error is forwarded to the subscription-error channel.
// The dest channel is kept open after stream error, in case re-subscribing is desired.
func StreamFallback[E any](fn func(ctx context.Context) (*E, error), frequency time.Duration, dest chan *E) (ethereum.Subscription, error) {
	return event.NewSubscription(func(quit <-chan struct{}) error {
		poll := time.NewTimer(frequency)
		defer poll.Stop()

		requestNext := make(chan struct{}, 1)

		getNext := func() error {
			ctx, cancel := context.WithTimeout(context.Background(), frequency)
			item, err := fn(ctx)
			cancel()
			if err != nil {
				var x gethrpc.Error
				if errors.As(err, &x); x.ErrorCode() == OutOfEventsErrCode {
					// back-off, by waiting for next tick, if out of events
					poll.Reset(frequency)
					return nil
				}
				return err
			}
			select {
			case dest <- item:
			case <-quit:
				return nil
			}
			requestNext <- struct{}{}
			return nil
		}

		// immediately start pulling data
		requestNext <- struct{}{}

		for {
			select {
			case <-quit:
				return nil
			case <-poll.C:
				if err := getNext(); err != nil {
					return err
				}
			case <-requestNext:
				if err := getNext(); err != nil {
					return err
				}
			}
		}
	}), nil
}

// Subscriber implements the subscribe subset of the RPC client interface.
// The inner geth-native Subscribe interface returns a struct subscription type,
// this can be interpreted as general ethereum.Subscription but may require a wrapper,
// like in the op-service client package.
type Subscriber interface {
	Subscribe(ctx context.Context, namespace string, channel any, args ...any) (ethereum.Subscription, error)
}

// ErrClosedByServer is sent over the subscription error-channel by Subscribe when the server closes the subscription.
var ErrClosedByServer = errors.New("closed by server")

// SubscribeStream subscribes to a Stream.
// This may return a gethrpc.ErrNotificationsUnsupported error, if subscriptions over RPC are not supported.
// The client should then fall back to manual RPC polling, with OutOfEventsErrCode error checks.
// The returned subscription has an error channel, which may send a ErrClosedByServer when the server closes the subscription intentionally.
// Or any of the geth RPC errors, when the connection closes or RPC fails.
// The args work like the Subscriber interface: the subscription identifier needs to be there.
func SubscribeStream[E any](ctx context.Context, namespace string, subscriber Subscriber, dest chan *E, args ...any) (ethereum.Subscription, error) {
	unpackCh := make(chan EventEntry[E])
	sub, err := subscriber.Subscribe(ctx, namespace, unpackCh, args...)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer close(dest)

		for {
			select {
			case <-quit: // when client wants to quit
				sub.Unsubscribe()
				return nil
			case err := <-sub.Err(): // when RPC fails / closes
				return err
			case x := <-unpackCh:
				if x.Data == nil { // when server wants us to quit.
					sub.Unsubscribe() // be nice, clean up the subscription.
					return ErrClosedByServer
				}
				select {
				case <-quit:
					return nil
				case dest <- x.Data:
				}
			}
		}
	}), nil
}

// Stream is a queue of events (wrapped objects) that can be pulled from or subscribed to via RPC.
// When subscribed, no data is queued, and sent proactively to the client instead (e.g. over websocket).
// If not subscribed, data can be served one by one manually (e.g. polled over HTTP).
// At most one concurrent subscription is supported.
type Stream[E any] struct {
	log log.Logger

	// queue buffers events until they are pulled manually.
	// No events are buffered if an RPC subscription is active.
	queue []*E

	// maxQueueSize is the maximum number of events that we retain for manual polling.
	// The oldest events are dropped first.
	maxQueueSize int

	// sub is the active RPC subscription we direct all events to.
	// sub may be nil, in which case we buffer events for manual reading (HTTP polling).
	// if notify errors, the notifier is broken, and should be dropped.
	sub      *gethrpc.Subscription
	notifier *gethrpc.Notifier

	mu sync.Mutex
}

// NewStream creates a new Stream.
// With a maxQueueSize, to limit how many events are buffered. The oldest events are dropped first, if overflowing.
func NewStream[E any](log log.Logger, maxQueueSize int) *Stream[E] {
	return &Stream[E]{
		log:          log,
		maxQueueSize: maxQueueSize,
	}
}

// notify is a helper func to send an event entry to the active subscription.
func (evs *Stream[E]) notify(v EventEntry[E]) {
	if evs.sub == nil {
		return
	}
	err := evs.notifier.Notify(evs.sub.ID, v)
	if err != nil {
		evs.log.Debug("Failed to notify, closing subscription now.", "err", err)
		evs.sub = nil
		evs.notifier = nil
	}
}

// Subscribe opens an RPC subscription that will be served with all future events.
// Previously buffered events will all be dropped.
func (evs *Stream[E]) Subscribe(ctx context.Context) (*gethrpc.Subscription, error) {
	evs.mu.Lock()
	defer evs.mu.Unlock()

	notifier, supported := gethrpc.NotifierFromContext(ctx)
	if !supported {
		return &gethrpc.Subscription{}, gethrpc.ErrNotificationsUnsupported
	}
	rpcSub := notifier.CreateSubscription()

	evs.sub = rpcSub
	evs.notifier = notifier
	evs.queue = nil // Now that there is a subscription, no longer buffer anything.

	// close when client closes the subscription
	go func() {
		// Errors when connection is disrupted/closed.
		// Closed when subscription is over.
		clErr := <-rpcSub.Err()
		if clErr != nil {
			if errors.Is(clErr, gethrpc.ErrClientQuit) {
				evs.log.Debug("RPC client disconnected, closing subscription")
			} else {
				evs.log.Warn("Subscription error", "err", clErr)
			}
		}
		evs.mu.Lock()
		defer evs.mu.Unlock()
		if evs.sub == rpcSub { // if we still maintain this same subscription, unregister it.
			evs.sub = nil
			evs.notifier = nil
		}
	}()

	return rpcSub, nil
}

// closeSub closes the active subscription, if any.
func (evs *Stream[E]) closeSub() {
	if evs.sub == nil {
		return
	}
	// Let the subscription know we're no longer serving them
	evs.notify(EventEntry[E]{Data: nil, Close: true})
	// Note: the connection stays open,
	// a subscription is just the choice of the server to write function-calls back with a particular RPC ID.
	// The server ends up holding on to an error channel,
	// namespace string, and RPC ID, until the client connection closes.
	// We have no way of cleaning this up from the server-side without geth-diff.

	evs.sub = nil
	evs.notifier = nil
}

// Serve serves a single event. It will return a JSON-RPC error with code OutOfEventsErrCode
// if no events are available to pull at this time.
// Serve will close any active subscription,
// as manual event retrieval and event-subscription are mutually exclusive modes.
func (evs *Stream[E]) Serve() (*E, error) {
	evs.mu.Lock()
	defer evs.mu.Unlock()
	// If we switch to manual event reading, cancel any open event subscription,
	// we don't want to push events over a subscription at the same time as a client is pulling.
	evs.closeSub()
	if len(evs.queue) == 0 {
		return nil, &gethrpc.JsonError{
			Code:    OutOfEventsErrCode,
			Message: "out of events",
		}
	}
	item := evs.queue[0]
	// evs.queue backing array will run out of capacity at some point on append(),
	// then re-allocate, and free what we dropped from the start.
	evs.queue = evs.queue[1:]
	return item, nil
}

// Send will send an event, either by enqueuing it for later retrieval,
// or by directly sending it to an active subscription.
func (evs *Stream[E]) Send(ev *E) {
	evs.mu.Lock()
	defer evs.mu.Unlock()
	if evs.sub != nil {
		evs.notify(EventEntry[E]{
			Data: ev,
		})
		return
	}
	evs.queue = append(evs.queue, ev)
	if overflow := len(evs.queue) - evs.maxQueueSize; overflow > 0 {
		evs.log.Warn("Event queue filled up, dropping oldest events", "overflow", overflow)
		evs.queue = slices.Delete(evs.queue, 0, overflow)
	}
}
