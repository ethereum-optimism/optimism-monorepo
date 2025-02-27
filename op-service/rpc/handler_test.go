package rpc

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/ethereum-optimism/optimism/op-service/testlog"
)

func TestHandler(t *testing.T) {
	logger := testlog.Logger(t, log.LevelInfo)
	h := NewHandler("v1.2.3", WithLogger(logger))
	t.Cleanup(h.Stop)

	rpcEntry := rpc.API{
		Namespace: "foo",
		Service:   new(testAPI),
	}

	require.ErrorContains(t, h.AddRPC("/"), "suffix")
	require.ErrorContains(t, h.AddRPC(""), "already exists")
	require.ErrorContains(t, h.AddAPIToRPC("/extra", rpcEntry), "not found")
	require.NoError(t, h.AddRPC("/extra"))
	require.NoError(t, h.AddAPIToRPC("/extra", rpcEntry))

	// WS-RPC / HTTP-RPC / health are tested in server_test.go
}
