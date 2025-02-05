package client

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/rpc"
)

type mockServer struct {
	count int
}

func (m *mockServer) Count() {
	m.count += 1
}

func TestLazyRPC(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	addr := listener.Addr().String()

	cl := newLazyRPC("ws://"+addr, applyOptions(nil))
	defer cl.Close()

	// At this point the connection is online, but the RPC is not.
	// RPC request attempts should fail.
	{
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*4)
		attempt1Err := cl.CallContext(ctx, nil, "foo_count")
		cancel()
		require.ErrorContains(t, attempt1Err, "i/o timeout")
		require.NotNil(t, ctx.Err())
	}

	// Now let's serve a websocket RPC
	rpcSrv := rpc.NewServer()
	defer rpcSrv.Stop()
	wsHandler := rpcSrv.WebsocketHandler([]string{"*"})
	httpSrv := &http.Server{Handler: wsHandler}
	defer httpSrv.Close()

	go func() {
		_ = httpSrv.Serve(listener) // always non-nil, returned when server exits.
	}()

	ms := &mockServer{}
	require.NoError(t, node.RegisterApis([]rpc.API{{
		Namespace: "foo",
		Service:   ms,
	}}, nil, rpcSrv))

	// and see if the lazy-dial client can reach it
	require.Equal(t, 0, ms.count)
	attempt2Err := cl.CallContext(context.Background(), nil, "foo_count")
	require.NoError(t, attempt2Err)
	require.Equal(t, 1, ms.count)
}
