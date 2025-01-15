package bootstrap

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/opcm"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/standard"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/testutil"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/retryproxy"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-service/testutils/anvil"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/stretchr/testify/require"
)

func TestImplementations(t *testing.T) {
	for _, network := range networks {
		t.Run(network, func(t *testing.T) {
			envVar := strings.ToUpper(network) + "_RPC_URL"
			rpcURL := os.Getenv(envVar)
			require.NotEmpty(t, rpcURL, "must specify RPC url via %s env var", envVar)
			testImplementations(t, rpcURL)
		})
	}
}

func testImplementations(t *testing.T, forkRPCURL string) {
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

	client, err := ethclient.Dial(runner.RPCUrl())
	require.NoError(t, err)

	chainID, err := client.ChainID(ctx)
	require.NoError(t, err)

	superchain, err := standard.SuperchainFor(chainID.Uint64())
	require.NoError(t, err)

	loc, _ := testutil.LocalArtifacts(t)

	deploy := func() opcm.DeployImplementationsOutput {
		out, err := Implementations(ctx, ImplementationsConfig{
			L1RPCUrl:                        runner.RPCUrl(),
			PrivateKey:                      "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
			ArtifactsLocator:                loc,
			Logger:                          lgr,
			L1ContractsRelease:              "dev",
			WithdrawalDelaySeconds:          standard.WithdrawalDelaySeconds,
			MinProposalSizeBytes:            standard.MinProposalSizeBytes,
			ChallengePeriodSeconds:          standard.ChallengePeriodSeconds,
			ProofMaturityDelaySeconds:       standard.ProofMaturityDelaySeconds,
			DisputeGameFinalityDelaySeconds: standard.DisputeGameFinalityDelaySeconds,
			MIPSVersion:                     1,
			SuperchainConfigProxy:           common.Address(*superchain.Config.SuperchainConfigAddr),
			ProtocolVersionsProxy:           common.Address(*superchain.Config.ProtocolVersionsAddr),
			UseInterop:                      false,
		})
		require.NoError(t, err)
		return out
	}

	// Assert that addresses stay the same between runs
	deployment1 := deploy()
	deployment2 := deploy()
	require.Equal(t, deployment1, deployment2)
}
