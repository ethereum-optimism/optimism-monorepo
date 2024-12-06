package bootstrap

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/retryproxy"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-service/testutils/anvil"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/stretchr/testify/require"
)

var networks = []string{"mainnet", "sepolia"}

var versions = []string{"v1.8.0-rc.3"}

func TestOPCMLiveChain(t *testing.T) {
	for _, network := range networks {
		for _, version := range versions {
			t.Run(network+"-"+version, func(t *testing.T) {
				if version == "v1.8.0-rc.3" && network == "mainnet" {
					t.Skip("v1.8.0-rc.3 not supported on mainnet yet")
				}

				envVar := strings.ToUpper(network) + "_RPC_URL"
				rpcURL := os.Getenv(envVar)
				require.NotEmpty(t, rpcURL, "must specify RPC url via %s env var", envVar)
				testOPCMLiveChain(t, "op-contracts/"+version, rpcURL)
			})
		}
	}
}

func testOPCMLiveChain(t *testing.T, version string, forkRPCURL string) {
	t.Parallel()

	if forkRPCURL == "" {
		t.Skip("forkRPCURL not set")
	}

	lgr := testlog.Logger(t, slog.LevelDebug)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	retryProxy := retryproxy.New(lgr, forkRPCURL)
	require.NoError(t, retryProxy.Start())
	t.Cleanup(func() {
		require.NoError(t, retryProxy.Stop())
	})

	runner, err := anvil.New(
		retryProxy.Endpoint(),
		lgr,
	)
	require.NoError(t, err)

	require.NoError(t, runner.Start(ctx))
	t.Cleanup(func() {
		require.NoError(t, runner.Stop())
	})

	out, err := OPCM(ctx, OPCMConfig{
		L1RPCUrl:   runner.RPCUrl(),
		PrivateKey: "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
		Release:    version,
		Logger:     lgr,
	})
	require.NoError(t, err)
	require.NotEmpty(t, out.Opcm)

	client, err := ethclient.Dial(runner.RPCUrl())
	require.NoError(t, err)
	code, err := client.CodeAt(ctx, out.Opcm, nil)
	require.NoError(t, err)
	require.NotEmpty(t, code)
}
