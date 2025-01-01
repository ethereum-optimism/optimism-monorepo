package rpc

import (
	"context"
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
}

func (api *testStreamRPC) Foo(ctx context.Context) (*rpc.Subscription, error) {
	return api.events.Subscribe(ctx)
}

func (api *testStreamRPC) PullFoo() (*Foo, error) {
	return api.events.Serve()
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
	sub, err = SubscribeStream[Foo](testCtx,
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
