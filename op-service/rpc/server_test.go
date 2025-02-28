package rpc

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/ethereum-optimism/optimism/op-service/testlog"
)

type testAPI struct{}

func (t *testAPI) Frobnicate(n int) int {
	return n * 2
}

func TestBaseServer(t *testing.T) {
	appVersion := "test"
	logger := testlog.Logger(t, log.LevelTrace)
	server := ServerFromConfig(&ServerConfig{
		HttpOptions: nil,
		RpcOptions: []Option{
			WithLogger(logger),
			WithWebsocketEnabled(),
		},
		Host:       "127.0.0.1",
		Port:       0,
		AppVersion: appVersion,
	})
	server.AddAPI(rpc.API{
		Namespace: "test",
		Service:   new(testAPI),
	})
	require.NoError(t, server.Start(), "must start")

	t.Cleanup(func() {
		err := server.Stop()
		if err != nil {
			panic(err)
		}
	})

	t.Run("supports 0 port", func(t *testing.T) {
		_, portStr, err := net.SplitHostPort(server.Endpoint())
		require.NoError(t, err)
		port, err := strconv.Atoi(portStr)
		require.NoError(t, err)
		require.Greater(t, port, 0)
	})

	require.NoError(t, server.AddRPC("/extra"))
	require.NoError(t, server.AddAPIToRPC("/extra", rpc.API{
		Namespace: "test2",
		Service:   new(testAPI),
	}))

	t.Run("regular", func(t *testing.T) {
		testServer(t, server.Endpoint(), appVersion, "test")
	})
	t.Run("extra route", func(t *testing.T) {
		testServer(t, server.Endpoint()+"/extra", appVersion, "test2")
	})
}

func testServer(t *testing.T, endpoint string, appVersion string, namespace string) {

	httpRPCClient, err := rpc.Dial("http://" + endpoint)
	require.NoError(t, err)
	t.Cleanup(httpRPCClient.Close)

	t.Run("supports GET /healthz", func(t *testing.T) {
		res, err := http.Get("http://" + endpoint + "/healthz")
		require.NoError(t, err)
		defer res.Body.Close()
		body, err := io.ReadAll(res.Body)
		require.NoError(t, err)
		require.EqualValues(t, fmt.Sprintf("{\"version\":\"%s\"}\n", appVersion), string(body))
	})

	t.Run("supports health_status", func(t *testing.T) {
		var res string
		require.NoError(t, httpRPCClient.Call(&res, "health_status"))
		require.Equal(t, appVersion, res)
	})

	t.Run("supports additional RPC APIs", func(t *testing.T) {
		var res int
		require.NoError(t, httpRPCClient.Call(&res, namespace+"_frobnicate", 2))
		require.Equal(t, 4, res)
	})

	t.Run("supports websocket", func(t *testing.T) {
		wsEndpoint := "ws://" + endpoint
		t.Log("connecting to", wsEndpoint)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()
		wsCl, err := rpc.DialContext(ctx, wsEndpoint)
		require.NoError(t, err)
		t.Cleanup(wsCl.Close)
		var res int
		require.NoError(t, wsCl.Call(&res, namespace+"_frobnicate", 42))
		require.Equal(t, 42*2, res)
	})
}

// TestUserMiddlewareBeforeHealth tests that the health endpoint is always available, in front of user-middleware.
func TestUserMiddlewareBeforeHealth(t *testing.T) {
	appVersion := "test"
	logger := testlog.Logger(t, log.LevelTrace)
	server := ServerFromConfig(&ServerConfig{
		HttpOptions: nil,
		RpcOptions: []Option{
			WithLogger(logger),
			WithWebsocketEnabled(),
			WithMiddleware(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusTeapot)
				})
			}),
		},
		Host:       "127.0.0.1",
		Port:       0,
		AppVersion: appVersion,
	})
	server.AddAPI(rpc.API{
		Namespace: "test",
		Service:   new(testAPI),
	})
	require.NoError(t, server.Start(), "must start")

	t.Cleanup(func() {
		err := server.Stop()
		if err != nil {
			panic(err)
		}
	})

	t.Run("does not support other GET /foobar", func(t *testing.T) {
		res, err := http.Get(server.httpServer.HTTPEndpoint() + "/foobar")
		require.NoError(t, err)
		defer res.Body.Close()
		require.Equal(t, http.StatusTeapot, res.StatusCode)
	})

	t.Run("supports GET /healthz", func(t *testing.T) {
		res, err := http.Get(server.httpServer.HTTPEndpoint() + "/healthz")
		require.NoError(t, err)
		defer res.Body.Close()
		require.Equal(t, http.StatusOK, res.StatusCode)
		body, err := io.ReadAll(res.Body)
		require.NoError(t, err)
		require.EqualValues(t, fmt.Sprintf("{\"version\":\"%s\"}\n", appVersion), string(body))
	})

}

func TestAuthServer(t *testing.T) {
	secret := [32]byte{0: 4}
	badSecret := [32]byte{0: 5}

	appVersion := "test"
	logger := testlog.Logger(t, log.LevelTrace)
	server := ServerFromConfig(&ServerConfig{
		HttpOptions: nil,
		RpcOptions: []Option{
			WithLogger(logger),
			WithWebsocketEnabled(),
			WithJWTSecret(secret[:]),
		},
		Host:       "127.0.0.1",
		Port:       0,
		AppVersion: appVersion,
	})
	server.AddAPI(rpc.API{
		Namespace: "test",
		Service:   new(testAPI),
	})
	require.NoError(t, server.Start(), "must start")

	// verify we can add routes after Start() while we are at it
	require.NoError(t, server.AddRPC("/other"))
	require.NoError(t, server.AddAPIToRPC("/other", rpc.API{
		Namespace: "test",
		Service:   new(testAPI),
	}))

	t.Cleanup(func() {
		err := server.Stop()
		if err != nil {
			panic(err)
		}
	})

	testAuth := func(t *testing.T, endpoint string) {
		httpRPCClient, err := rpc.DialOptions(context.Background(), "http://"+endpoint,
			rpc.WithHTTPAuth(node.NewJWTAuth(secret)))
		require.NoError(t, err)
		t.Cleanup(httpRPCClient.Close)

		var res int
		require.NoError(t, httpRPCClient.Call(&res, "test_frobnicate", 2))
		require.Equal(t, 4, res)

		httpRPCClient2, err := rpc.DialOptions(context.Background(), "http://"+endpoint,
			rpc.WithHTTPAuth(node.NewJWTAuth(badSecret)))
		require.NoError(t, err)
		t.Cleanup(httpRPCClient2.Close)

		var resNo int
		require.Error(t, httpRPCClient2.Call(&resNo, "test_frobnicate", 10),
			"http is lazy-auth and should error with bad secret on first call")

		_, err = rpc.DialOptions(context.Background(), "ws://"+endpoint,
			rpc.WithHTTPAuth(node.NewJWTAuth(badSecret)))
		require.Error(t, err, "websocket is immediate auth and should error with bad secret on dial")

		wsRPCClient, err := rpc.DialOptions(context.Background(), "ws://"+endpoint,
			rpc.WithHTTPAuth(node.NewJWTAuth(secret)))
		require.NoError(t, err)
		t.Cleanup(wsRPCClient.Close)

		var res2 int
		require.NoError(t, wsRPCClient.Call(&res2, "test_frobnicate", 3))
		require.Equal(t, 6, res2)
	}
	t.Run("regular", func(t *testing.T) {
		testAuth(t, server.Endpoint())
	})
	// auth should be applied to all routes that serve RPC
	t.Run("other", func(t *testing.T) {
		testAuth(t, server.Endpoint()+"/other")
	})
}
