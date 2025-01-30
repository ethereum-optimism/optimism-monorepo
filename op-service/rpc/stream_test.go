package rpc

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/ethereum-optimism/optimism/op-service/testlog"
)

type Foo struct {
	Message string `json:"message"`
}

type testStreamRPC struct {
	log    log.Logger
	events *Stream[Foo]

	// To await out-of-events case.
	outOfEvents chan struct{}
}

func (api *testStreamRPC) Foo(ctx context.Context) (*rpc.Subscription, error) {
	return api.events.Subscribe(ctx)
}

func (api *testStreamRPC) PullFoo() (*Foo, error) {
	data, err := api.events.Serve()
	if api.outOfEvents != nil && err != nil {
		var x rpc.Error
		if errors.As(err, &x); x.ErrorCode() == OutOfEventsErrCode {
			api.outOfEvents <- struct{}{}
		}
	}
	return data, err
}

func TestStream_Polling(t *testing.T) {
	logger := testlog.Logger(t, log.LevelDebug)
	server := rpc.NewServer()
	t.Cleanup(server.Stop)

	maxQueueSize := 10
	api := &testStreamRPC{
		log:    logger,
		events: NewStream[Foo](logger, maxQueueSize),
	}
	require.NoError(t, server.RegisterName("custom", api))

	cl := rpc.DialInProc(server)
	t.Cleanup(cl.Close)

	// Initially no data is there
	var x *Foo
	var jsonErr rpc.Error
	require.ErrorAs(t, cl.Call(&x, "custom_pullFoo"), &jsonErr, "expecting json error")
	require.Equal(t, OutOfEventsErrCode, jsonErr.ErrorCode())
	require.Equal(t, "out of events", jsonErr.Error())
	require.Nil(t, x)

	x = nil
	jsonErr = nil

	// send two events: these will be buffered
	api.events.Send(&Foo{Message: "hello alice"})
	api.events.Send(&Foo{Message: "hello bob"})

	require.NoError(t, cl.Call(&x, "custom_pullFoo"))
	require.Equal(t, "hello alice", x.Message)
	x = nil

	// can send more, while not everything has been read yet.
	api.events.Send(&Foo{Message: "hello charlie"})

	require.NoError(t, cl.Call(&x, "custom_pullFoo"))
	require.Equal(t, "hello bob", x.Message)
	x = nil

	require.NoError(t, cl.Call(&x, "custom_pullFoo"))
	require.Equal(t, "hello charlie", x.Message)
	x = nil

	// out of events again
	require.ErrorAs(t, cl.Call(&x, "custom_pullFoo"), &jsonErr, "expecting json error")
	require.Equal(t, OutOfEventsErrCode, jsonErr.ErrorCode())
	require.Equal(t, "out of events", jsonErr.Error())
	require.Nil(t, x)

	// now send 1 too many events
	for i := 0; i <= maxQueueSize; i++ {
		api.events.Send(&Foo{Message: fmt.Sprintf("hello %d", i)})
	}

	require.NoError(t, cl.Call(&x, "custom_pullFoo"))
	require.Equal(t, "hello 1", x.Message, "expecting entry 0 to be dropped")
}

type ClientWrapper struct {
	cl *rpc.Client
}

func (c *ClientWrapper) Subscribe(ctx context.Context, namespace string, channel any, args ...any) (ethereum.Subscription, error) {
	return c.cl.Subscribe(ctx, namespace, channel, args...)
}

var _ Subscriber = (*ClientWrapper)(nil)

func TestStream_Subscription(t *testing.T) {
	logger := testlog.Logger(t, log.LevelDebug)
	server := rpc.NewServer()
	t.Cleanup(server.Stop)

	testCtx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	maxQueueSize := 10
	api := &testStreamRPC{
		log:    logger,
		events: NewStream[Foo](logger, maxQueueSize),
	}
	require.NoError(t, server.RegisterName("custom", api))

	cl := rpc.DialInProc(server)
	t.Cleanup(cl.Close)

	dest := make(chan *Foo, 10)
	sub, err := SubscribeStream[Foo](testCtx,
		"custom", &ClientWrapper{cl: cl}, dest, "foo")
	require.NoError(t, err)

	api.events.Send(&Foo{Message: "hello alice"})
	api.events.Send(&Foo{Message: "hello bob"})
	select {
	case x := <-dest:
		require.Equal(t, "hello alice", x.Message)
	case <-testCtx.Done():
		t.Fatal("timed out subscription result")
	}
	select {
	case x := <-dest:
		require.Equal(t, "hello bob", x.Message)
	case <-testCtx.Done():
		t.Fatal("timed out subscription result")
	}

	// Now try and pull manually. This will cancel the subscription.
	var x *Foo
	var jsonErr rpc.Error
	require.ErrorAs(t, cl.Call(&x, "custom_pullFoo"), &jsonErr, "expecting json error")
	require.Equal(t, OutOfEventsErrCode, jsonErr.ErrorCode())
	require.Equal(t, "out of events", jsonErr.Error())
	require.Nil(t, x)

	// Server closes the subscription because we started polling instead.
	require.ErrorIs(t, ErrClosedByServer, <-sub.Err())
	require.Len(t, dest, 0)
	_, ok := <-dest
	require.False(t, ok, "dest is closed")

	// Send another event. This one will be buffered, because the subscription was stopped.
	api.events.Send(&Foo{Message: "hello charlie"})

	require.NoError(t, cl.Call(&x, "custom_pullFoo"))
	require.Equal(t, "hello charlie", x.Message)

	// And one more, buffered, but not read. Instead, we open a new subscription.
	// We expect this to be dropped. Subscriptions only provide live data.
	api.events.Send(&Foo{Message: "hello dave"})

	dest = make(chan *Foo, 10)
	_, err = SubscribeStream[Foo](testCtx,
		"custom", &ClientWrapper{cl: cl}, dest, "foo")
	require.NoError(t, err)

	// Send another event, now that we have a live subscription again.
	api.events.Send(&Foo{Message: "hello elizabeth"})

	select {
	case x := <-dest:
		require.Equal(t, "hello elizabeth", x.Message)
	case <-testCtx.Done():
		t.Fatal("timed out subscription result")
	}
}

func TestStreamFallback(t *testing.T) {
	appVersion := "test"

	logger := testlog.Logger(t, log.LevelDebug)

	maxQueueSize := 10
	api := &testStreamRPC{
		log:         logger,
		events:      NewStream[Foo](logger, maxQueueSize),
		outOfEvents: make(chan struct{}, 100),
	}
	// Create an HTTP server, this won't support RPC subscriptions
	server := NewServer(
		"127.0.0.1",
		0,
		appVersion,
		WithLogger(logger),
		WithAPIs([]rpc.API{
			{
				Namespace: "custom",
				Service:   api,
			},
		}),
	)
	require.NoError(t, server.Start(), "must start")

	// Dial via HTTP, to ensure no subscription support
	rpcClient, err := rpc.Dial(fmt.Sprintf("http://%s", server.endpoint))
	require.NoError(t, err)
	t.Cleanup(rpcClient.Close)

	testCtx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	// regular subscription won't work over HTTP
	dest := make(chan *Foo, 10)
	_, err = SubscribeStream[Foo](testCtx,
		"custom", &ClientWrapper{cl: rpcClient}, dest, "foo")
	require.ErrorIs(t, err, rpc.ErrNotificationsUnsupported, "no subscriptions")

	// Fallback will work, and pull the buffered stream data
	fn := func(ctx context.Context) (*Foo, error) {
		var x *Foo
		err := rpcClient.CallContext(ctx, &x, "custom_pullFoo")
		return x, err
	}
	sub, err := StreamFallback[Foo](fn, time.Millisecond*200, dest)
	require.NoError(t, err)

	api.events.Send(&Foo{"hello world"})

	select {
	case err := <-sub.Err():
		require.NoError(t, err, "unexpected subscription error")
	case x := <-dest:
		require.Equal(t, "hello world", x.Message)
	case <-testCtx.Done():
		t.Fatal("test timeout")
	}

	// Ensure we hit the out-of-events error intermittently
	select {
	case <-api.outOfEvents:
	case <-testCtx.Done():
		t.Fatal("test timeout while waiting for out-of-events")
	}
	// Now send an event, which will only be picked up after backoff is over,
	// since we just ran into out-of-events.
	api.events.Send(&Foo{"hello again"})

	// Wait for polling to pick up the data
	select {
	case err := <-sub.Err():
		require.NoError(t, err, "unexpected subscription error")
	case x := <-dest:
		require.Equal(t, "hello again", x.Message)
	case <-testCtx.Done():
		t.Fatal("test timeout")
	}

	sub.Unsubscribe()
	dest <- &Foo{Message: "open check"}
	_, ok := <-dest
	require.True(t, ok, "kept open for easy resubscribing")
}
