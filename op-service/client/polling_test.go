package client

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

type MockRPC struct {
	t           *testing.T
	callResults []*callResult
	mtx         sync.RWMutex
	callCount   int
	autopop     bool
	closed      bool
}

type callResult struct {
	root  common.Hash
	error error
}

func (m *MockRPC) Close() {
	m.closed = true
}

func (m *MockRPC) CallContext(ctx context.Context, result any, method string, args ...any) error {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	if method != "eth_getBlockByNumber" {
		m.t.Fatalf("invalid method %s", method)
	}
	if args[0] != "latest" {
		m.t.Fatalf("invalid arg %v", args[0])
	}

	m.callCount++
	res := m.callResults[0]
	headerP := result.(**types.Header)
	*headerP = &types.Header{
		Root: res.root,
	}
	if m.autopop {
		m.callResults = m.callResults[1:]
	}
	return res.error
}

func (m *MockRPC) BatchCallContext(ctx context.Context, b []rpc.BatchElem) error {
	m.t.Fatal("BatchCallContext should not be called")
	return nil
}

func (m *MockRPC) Subscribe(ctx context.Context, namespace string, channel any, args ...any) (ethereum.Subscription, error) {
	m.t.Fatal("Subscribe should not be called")
	return nil, nil
}

func (m *MockRPC) popResult() {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	m.callResults = m.callResults[1:]
}

func TestPollingClientSubscribeUnsubscribe(t *testing.T) {
	lgr := log.NewLogger(log.DiscardHandler())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	root1 := common.Hash{0x01}
	root2 := common.Hash{0x02}
	root3 := common.Hash{0x03}
	mockRPC := &MockRPC{
		t: t,
		callResults: []*callResult{
			{root1, nil},
			{root2, nil},
			{root3, nil},
		},
	}
	client := NewPollingClient(ctx, lgr, mockRPC, WithPollRate(0))

	subs := make([]ethereum.Subscription, 0)
	chans := make([]chan *types.Header, 0)
	for i := 0; i < 2; i++ {
		ch := make(chan *types.Header, 2)
		sub, err := doSubscribe(client, ch)
		require.NoError(t, err)
		subs = append(subs, sub)
		chans = append(chans, ch)
	}

	client.reqPoll()
	requireChansEqual(t, chans, root1)
	mockRPC.popResult()
	client.reqPoll()
	requireChansEqual(t, chans, root2)
	// Poll an additional time to show that responses with the same
	// data don't notify again.
	client.reqPoll()

	// Verify that no further notifications have been sent.
	for _, ch := range chans {
		select {
		case <-ch:
			t.Fatal("unexpected notification")
		case <-time.NewTimer(10 * time.Millisecond).C:
			continue
		}
	}

	mockRPC.popResult()
	subs[0].Unsubscribe()
	client.reqPoll()
	select {
	case <-chans[0]:
		t.Fatal("unexpected notification")
	case <-time.NewTimer(10 * time.Millisecond).C:
	}

	header := <-chans[1]
	require.Equal(t, root3, header.Root)
}

func TestPollingClientErrorRecovery(t *testing.T) {
	lgr := log.NewLogger(log.DiscardHandler())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	root := common.Hash{0x01}
	mockRPC := &MockRPC{
		t: t,
		callResults: []*callResult{
			{common.Hash{}, errors.New("foobar")},
			{common.Hash{}, errors.New("foobar")},
			{root, nil},
		},
		autopop: true,
	}
	client := NewPollingClient(ctx, lgr, mockRPC, WithPollRate(0))
	ch := make(chan *types.Header, 1)
	sub, err := doSubscribe(client, ch)
	require.NoError(t, err)
	defer sub.Unsubscribe()

	for i := 0; i < 3; i++ {
		client.reqPoll()
	}

	header := <-ch
	require.Equal(t, root, header.Root)
	require.Equal(t, 3, mockRPC.callCount)
}

func TestPollingClientClose(t *testing.T) {
	lgr := log.NewLogger(log.DiscardHandler())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	root := common.Hash{0x01}
	mockRPC := &MockRPC{
		t: t,
		callResults: []*callResult{
			{root, nil},
		},
		autopop: true,
	}
	client := NewPollingClient(ctx, lgr, mockRPC, WithPollRate(0))
	ch := make(chan *types.Header, 1)
	sub, err := doSubscribe(client, ch)
	require.NoError(t, err)
	client.reqPoll()
	header := <-ch
	cancel()
	require.Nil(t, <-sub.Err())
	require.Equal(t, root, header.Root)
	require.Equal(t, 1, mockRPC.callCount)

	// unsubscribe should be safe
	sub.Unsubscribe()

	_, err = doSubscribe(client, ch)
	require.Equal(t, ErrSubscriberClosed, err)
}

func requireChansEqual(t *testing.T, chans []chan *types.Header, root common.Hash) {
	t.Helper()
	for _, ch := range chans {
		header := <-ch
		require.Equal(t, root, header.Root)
	}
}

func doSubscribe(client RPC, ch chan<- *types.Header) (ethereum.Subscription, error) {
	return client.Subscribe(context.Background(), "eth", ch, "newHeads")
}
