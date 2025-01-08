package sources

import (
	"context"
	"net"
	"sync"

	"github.com/ethereum-optimism/optimism/op-service/client"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/rpc"
	"golang.org/x/sync/semaphore"
)

type limitClient struct {
	mutex  sync.Mutex
	closed bool
	c      client.RPC
	sema   *semaphore.Weighted
	wg     sync.WaitGroup
}

// joinWaitGroup will add the caller to the waitgroup if the client has not
// already been told to shutdown.  If the client has shut down, false is
// returned, otherwise true.
func (lc *limitClient) joinWaitGroup() bool {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()
	if lc.closed {
		return false
	}
	lc.wg.Add(1)
	return true
}

// LimitRPC limits concurrent RPC requests (excluding subscriptions) to a given number by wrapping the client with a semaphore.
func LimitRPC(c client.RPC, concurrentRequests int) client.RPC {
	return &limitClient{
		c: c,
		// the capacity of the channel determines how many go-routines can concurrently execute requests with the wrapped client.
		sema: semaphore.NewWeighted(int64(concurrentRequests)),
	}
}

func (lc *limitClient) BatchCallContext(ctx context.Context, b []rpc.BatchElem) error {
	if !lc.joinWaitGroup() {
		return net.ErrClosed
	}
	defer lc.wg.Done()
	if err := lc.sema.Acquire(ctx, 1); err != nil {
		return err
	}
	defer lc.sema.Release(1)
	return lc.c.BatchCallContext(ctx, b)
}

func (lc *limitClient) CallContext(ctx context.Context, result any, method string, args ...any) error {
	if !lc.joinWaitGroup() {
		return net.ErrClosed
	}
	defer lc.wg.Done()
	if err := lc.sema.Acquire(ctx, 1); err != nil {
		return err
	}
	defer lc.sema.Release(1)
	return lc.c.CallContext(ctx, result, method, args...)
}

func (lc *limitClient) Subscribe(ctx context.Context, namespace string, channel any, args ...any) (ethereum.Subscription, error) {
	if !lc.joinWaitGroup() {
		return nil, net.ErrClosed
	}
	defer lc.wg.Done()
	// subscription doesn't count towards request limit
	return lc.c.Subscribe(ctx, namespace, channel, args...)
}

func (lc *limitClient) Close() {
	lc.mutex.Lock()
	lc.closed = true // No new waitgroup members after this is set
	lc.mutex.Unlock()
	lc.wg.Wait() // All waitgroup members exited, means no more dereferences of the client
	lc.c.Close()
}
