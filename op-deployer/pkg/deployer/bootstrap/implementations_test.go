package bootstrap

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-service/testutils/devnet"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/opcm"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/standard"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/testutil"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
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

	forkedL1, stopL1, err := devnet.NewForked(lgr, forkRPCURL)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, stopL1())
	})
	l1RPC := forkedL1.RPCUrl()

	client, err := ethclient.Dial(l1RPC)
	require.NoError(t, err)

	chainID, err := client.ChainID(ctx)
	require.NoError(t, err)

	superchain, err := standard.SuperchainFor(chainID.Uint64())
	require.NoError(t, err)

	loc, _ := testutil.LocalArtifacts(t)

	proxyAdminOwner, err := standard.L1ProxyAdminOwner(uint64(chainID.Uint64()))
	require.NoError(t, err)
	deploy := func() opcm.DeployImplementationsOutput {
		out, err := Implementations(ctx, ImplementationsConfig{
			L1RPCUrl:                        l1RPC,
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
			SuperchainConfigProxy:           superchain.SuperchainConfigAddr,
			ProtocolVersionsProxy:           superchain.ProtocolVersionsAddr,
			UpgradeController:               proxyAdminOwner,
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
