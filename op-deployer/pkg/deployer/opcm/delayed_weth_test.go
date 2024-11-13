package opcm

import (
	"math/big"
	"testing"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/broadcaster"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/standard"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/testutil"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/env"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"
)

func TestDeployDelayedWETH(t *testing.T) {
	_, artifacts := testutil.LocalArtifacts(t)

	host, err := env.DefaultScriptHost(
		broadcaster.NoopBroadcaster(),
		testlog.Logger(t, log.LevelInfo),
		common.Address{'D'},
		artifacts,
	)
	require.NoError(t, err)

	standardVersionsTOML, err := standard.L1VersionsDataFor(11155111)
	require.NoError(t, err)

	input := DeployDelayedWETHInput{
		Release:               "dev",
		StandardVersionsToml:  standardVersionsTOML,
		ProxyAdmin:            common.Address{'P'},
		SuperchainConfigProxy: common.Address{'S'},
		DelayedWethOwner:      common.Address{'O'},
		DelayedWethDelay:      big.NewInt(100),
	}

	output, err := DeployDelayedWETH(host, input)
	require.NoError(t, err)

	require.NotEmpty(t, output.DelayedWethImpl)
	require.NotEmpty(t, output.DelayedWethProxy)
}
