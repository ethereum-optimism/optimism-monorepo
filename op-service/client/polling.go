package client

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"
)

var ErrSubscriberClosed = errors.New("subscriber closed")

// PollingClient is an RPC client that provides newHeads subscriptions
// via a polling loop. It's designed for HTTP endpoints, but WS will
// work too.
type PollingClient struct {
	c        RPC
	lgr      log.Logger
	pollRate time.Duration
	ctx      context.Context
	cancel   context.CancelFunc
	currHead *types.Header
	subID    int

	// pollReqCh is used to request new polls of the upstream
	// RPC client.
	pollReqCh chan struct{}

	mtx sync.RWMutex

	subs map[int]chan *types.Header

	closedCh chan struct{}
}

type WrappedHTTPClientOption func(w *PollingClient)

// WithPollRate specifies the rate at which the PollingClient will poll
// for new heads. Setting this to zero disables polling altogether,
// which is useful for testing.
func WithPollRate(duration time.Duration) WrappedHTTPClientOption {
	return func(w *PollingClient) {
		w.pollRate = duration
	}
}

// NewPollingClient returns a new PollingClient. Canceling the passed-in context
// will close the client. Callers are responsible for closing the client in order
// to prevent resource leaks.
func NewPollingClient(ctx context.Context, lgr log.Logger, c RPC, opts ...WrappedHTTPClientOption) *PollingClient {
	ctx, cancel := context.WithCancel(ctx)
	res := &PollingClient{
		c:         c,
		lgr:       lgr,
		pollRate:  12 * time.Second,
		ctx:       ctx,
		cancel:    cancel,
		pollReqCh: make(chan struct{}, 1),
		subs:      make(map[int]chan *types.Header),
		closedCh:  make(chan struct{}),
	}
	for _, opt := range opts {
		opt(res)
	}
	go res.pollHeads()
	return res
}

// Close closes the PollingClient and the underlying RPC client it talks to.
func (w *PollingClient) Close() {
	w.cancel()
	<-w.closedCh
	w.c.Close()
}

func (w *PollingClient) CallContext(ctx context.Context, result any, method string, args ...any) error {
	return w.c.CallContext(ctx, result, method, args...)
}

func (w *PollingClient) BatchCallContext(ctx context.Context, b []rpc.BatchElem) error {
	return w.c.BatchCallContext(ctx, b)
}

// Subscribe supports eth_subscribe of block headers,
// by creating a new newHeads subscription. It takes identical arguments
// to Geth's native Subscribe method. It will return an error, however, if the
// passed in channel is not a *types.Headers channel or the subscription type is not
// newHeads. Or if the namespace is not "eth".
func (w *PollingClient) Subscribe(ctx context.Context, namespace string, channel any, args ...any) (ethereum.Subscription, error) {
	if namespace != "eth" {
		return nil, fmt.Errorf("polling fallback is only supported for eth_subscribe, not for namespace %q", namespace)
	}
	select {
	case <-w.ctx.Done():
		return nil, ErrSubscriberClosed
	default:
	}

	headerCh, ok := channel.(chan<- *types.Header)
	if !ok {
		return nil, errors.New("invalid channel type")
	}
	if len(args) != 1 {
		return nil, errors.New("invalid subscription args")
	}
	if args[0] != "newHeads" {
		return nil, errors.New("unsupported subscription type")
	}

	sub := make(chan *types.Header, 1)
	w.mtx.Lock()
	subID := w.subID
	w.subID++
	w.subs[subID] = sub
	w.mtx.Unlock()

	return event.NewSubscription(func(quit <-chan struct{}) error {
		for {
			select {
			case header := <-sub:
				headerCh <- header
			case <-quit:
				w.mtx.Lock()
				delete(w.subs, subID)
				w.mtx.Unlock()
				return nil
			case <-w.ctx.Done():
				return nil
			}
		}
	}), nil
}

func (w *PollingClient) pollHeads() {
	// To prevent polls from stacking up in case HTTP requests
	// are slow, use a similar model to the driver in which
	// polls are requested manually after each header is fetched.
	reqPollAfter := func() {
		if w.pollRate == 0 {
			return
		}
		time.AfterFunc(w.pollRate, w.reqPoll)
	}

	reqPollAfter()

	defer close(w.closedCh)

	for {
		select {
		case <-w.pollReqCh:
			// We don't need backoff here because we'll just try again
			// after the pollRate elapses.
			head, err := w.getLatestHeader()
			if err != nil {
				w.lgr.Error("error getting latest header", "err", err)
				reqPollAfter()
				continue
			}
			if w.currHead != nil && w.currHead.Hash() == head.Hash() {
				w.lgr.Trace("no change in head, skipping notifications")
				reqPollAfter()
				continue
			}

			w.lgr.Trace("notifying subscribers of new head", "head", head.Hash())
			w.currHead = head
			w.mtx.RLock()
			for _, sub := range w.subs {
				sub <- head
			}
			w.mtx.RUnlock()
			reqPollAfter()
		case <-w.ctx.Done():
			w.c.Close()
			return
		}
	}
}

func (w *PollingClient) getLatestHeader() (*types.Header, error) {
	ctx, cancel := context.WithTimeout(w.ctx, 5*time.Second)
	defer cancel()
	var head *types.Header
	err := w.CallContext(ctx, &head, "eth_getBlockByNumber", "latest", false)
	if err == nil && head == nil {
		err = ethereum.NotFound
	}
	return head, err
}

func (w *PollingClient) reqPoll() {
	w.pollReqCh <- struct{}{}
}
