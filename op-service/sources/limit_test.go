package sources

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-service/client"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

type MockRPC struct {
	t              *testing.T
	blockedCallers atomic.Int32
	errC           chan error
}

func (m *MockRPC) Close() {}

func (m *MockRPC) CallContext(ctx context.Context, result any, method string, args ...any) error {
	m.blockedCallers.Add(1)
	defer m.blockedCallers.Add(-1)
	return <-m.errC
}

func (m *MockRPC) BatchCallContext(ctx context.Context, b []rpc.BatchElem) error {
	m.blockedCallers.Add(1)
	defer m.blockedCallers.Add(-1)
	return <-m.errC
}

func (m *MockRPC) Subscribe(ctx context.Context, namespace string, channel any, args ...any) (ethereum.Subscription, error) {
	m.t.Fatal("Subscribe should not be called")
	return nil, nil
}

func asyncCallContext(ctx context.Context, lc client.RPC) chan error {
	errC := make(chan error)
	go func() {
		errC <- lc.CallContext(ctx, 0, "fake_method")
	}()
	return errC
}

func TestLimitClient(t *testing.T) {
	// The MockRPC will block all calls until errC is written to
	m := &MockRPC{
		t:    t,
		errC: make(chan error),
	}
	lc := LimitRPC(m, 2).(*limitClient)

	errC1 := asyncCallContext(context.Background(), lc)
	errC2 := asyncCallContext(context.Background(), lc)
	require.Eventually(t, func() bool { return m.blockedCallers.Load() == 2 }, time.Second, 10*time.Millisecond)

	// Once the limit of 2 clients has been reached, we enqueue two more,
	// one with a context that will expire
	tCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	errC3 := asyncCallContext(tCtx, lc)
	errC4 := asyncCallContext(context.Background(), lc)

	select {
	case err := <-errC3:
		require.ErrorIs(t, err, context.DeadlineExceeded)
	case <-time.After(time.Second):
		t.Fatalf("context should have expired and the call returned")
	}

	// No further clients should be allowed after this block, but existing
	// clients should persist until their contexts close
	go lc.Close()
	require.Eventually(t, func() bool {
		lc.mutex.Lock()
		defer lc.mutex.Unlock()
		return lc.closed
	}, time.Second, 10*time.Millisecond)

	err := lc.CallContext(context.Background(), 0, "fake_method")
	require.ErrorIs(t, err, net.ErrClosed, "Calls after close should return immediately with error")

	// Existing clients should all remain blocked
	select {
	case err := <-errC1:
		t.Fatalf("client should not have returned: %s", err)
	case err := <-errC2:
		t.Fatalf("client should not have returned: %s", err)
	case err := <-errC4:
		t.Fatalf("client should not have returned: %s", err)
	case <-time.After(50 * time.Millisecond):
		// None of the clients should return yet
	}

	m.errC <- errors.New("fake-error")
	m.errC <- errors.New("fake-error")
	require.Eventually(t, func() bool { return m.blockedCallers.Load() == 1 }, time.Second, 10*time.Millisecond)
	m.errC <- errors.New("fake-error")

	require.ErrorContains(t, <-errC1, "fake-error")
	require.ErrorContains(t, <-errC2, "fake-error")
	require.ErrorContains(t, <-errC4, "fake-error")

	require.Eventually(t, func() bool { return m.blockedCallers.Load() == 0 }, time.Second, 10*time.Millisecond)
}
