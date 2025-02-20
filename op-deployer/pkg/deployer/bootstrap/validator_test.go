package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/artifacts"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/testutil"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-service/testutils/devnet"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/stretchr/testify/require"
)

func TestValidator(t *testing.T) {
	for _, network := range networks {
		for _, release := range []string{"v1.8.0", "v2.0.0"} {
			t.Run(fmt.Sprintf("%s/%s", network, release), func(t *testing.T) {
				envVar := strings.ToUpper(network) + "_RPC_URL"
				rpcURL := os.Getenv(envVar)
				require.NotEmpty(t, rpcURL, "must specify RPC url via %s env var", envVar)
				loc, _ := testutil.LocalArtifacts(t)
				testValidator(t, rpcURL, loc, release)
			})
		}
	}
}

func testValidator(t *testing.T, forkRPCURL string, loc *artifacts.Locator, release string) {
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

	// Create the validator input configuration
	input := ValidatorInput{
		Release:                          release,
		SuperchainConfig:                 common.Address{'S'},
		ProtocolVersions:                 common.Address{'P'},
		L1PAOMultisig:                    common.Address{'M'},
		MIPS:                             common.Address{'I'},
		Challenger:                       common.Address{'C'},
		SuperchainConfigImpl:             common.Address{'1'},
		ProtocolVersionsImpl:             common.Address{'2'},
		L1ERC721BridgeImpl:               common.Address{'3'},
		OptimismPortalImpl:               common.Address{'4'},
		SystemConfigImpl:                 common.Address{'5'},
		OptimismMintableERC20FactoryImpl: common.Address{'6'},
		L1CrossDomainMessengerImpl:       common.Address{'7'},
		L1StandardBridgeImpl:             common.Address{'8'},
		DisputeGameFactoryImpl:           common.Address{'9'},
		AnchorStateRegistryImpl:          common.Address{'A'},
		DelayedWETHImpl:                  common.Address{'B'},
		MIPSImpl:                         common.Address{'D'},
	}

	out, err := Validator(ctx, ValidatorConfig{
		L1RPCUrl:         l1RPC,
		PrivateKey:       "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
		ArtifactsLocator: loc,
		Logger:           lgr,
		Input:            input,
	})
	require.NoError(t, err)

	client, err := ethclient.Dial(l1RPC)
	require.NoError(t, err)

	addr := out.Validator
	require.NotEmpty(t, addr)

	code, err := client.CodeAt(ctx, addr, nil)
	require.NoError(t, err)
	require.NotEmpty(t, code)
}
