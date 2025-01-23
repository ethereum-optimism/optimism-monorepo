package rpc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	gethevent "github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/ethereum-optimism/optimism/op-service/testlog"
)

type testSubscribeAPI struct {
	log log.Logger

	foo gethevent.FeedOf[int]
	bar gethevent.FeedOf[string]
}

func (api *testSubscribeAPI) Foo(ctx context.Context) (*rpc.Subscription, error) {
	return SubscribeRPC(ctx, api.log, &api.foo)
}

func (api *testSubscribeAPI) Bar(ctx context.Context) (*rpc.Subscription, error) {
	return SubscribeRPC(ctx, api.log, &api.bar)
}

func (api *testSubscribeAPI) GreetName(ctx context.Context, name string) (*rpc.Subscription, error) {
	return nil, &rpc.JsonError{
		Code:    -100_000,
		Message: "hello " + name,
		Data:    nil,
	}
}

func TestSubscribeRPC(t *testing.T) {
	logger := testlog.Logger(t, log.LevelDebug)
	server := rpc.NewServer()
	api := &testSubscribeAPI{
		log: logger,
	}
	require.NoError(t, server.RegisterName("custom", api))

	cl := rpc.DialInProc(server)

	// Set up Foo subscription
	ctx, fooCancel := context.WithCancel(context.Background())
	fooCh := make(chan int, 10)
	fooSub, err := cl.Subscribe(ctx, "custom", fooCh, "foo")
	require.NoError(t, err)
	api.foo.Send(123)
	api.foo.Send(42)
	api.bar.Send("x") // will be missed, we are not yet subscribed to "bar"
	api.foo.Send(10)
	for _, v := range []int{123, 42, 10} {
		select {
		case x := <-fooCh:
			require.Equal(t, v, x)
		case err := <-fooSub.Err():
			require.NoError(t, err)
		}
	}

	// setup Bar subscription
	ctx, barCancel := context.WithCancel(context.Background())
	barCh := make(chan string, 10)
	barSub, err := cl.Subscribe(ctx, "custom", barCh, "bar")
	require.NoError(t, err)
	api.bar.Send("a")
	api.foo.Send(20) // must not interfere
	api.bar.Send("b")
	api.bar.Send("c")
	// "x" was sent before this subscription became active
	for _, v := range []string{"a", "b", "c"} {
		select {
		case x := <-barCh:
			require.Equal(t, v, x)
		case err := <-barSub.Err():
			require.NoError(t, err)
		}
	}

	// pick up the item from Foo that we ignored in Bar
	select {
	case x := <-fooCh:
		require.Equal(t, 20, x)
	case err := <-fooSub.Err():
		require.NoError(t, err)
	}

	barSub.Unsubscribe()
	barCancel()

	fooSub.Unsubscribe()
	fooCancel()

	// Try with a function argument, verify the function arg works
	ctx, greetCancel := context.WithCancel(context.Background())
	greetCh := make(chan string, 10)
	_, err = cl.Subscribe(ctx, "custom", greetCh, "greetName", "alice")
	var x rpc.Error
	require.ErrorAs(t, err, &x)
	require.Equal(t, x.Error(), "hello alice")

	greetCancel()
}
