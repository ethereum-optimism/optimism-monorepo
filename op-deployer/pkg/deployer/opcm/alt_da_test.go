package opcm

import (
	"math/big"
	"testing"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/broadcaster"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/testutil"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/env"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"
)

func TestDeployAltDA(t *testing.T) {
	_, artifacts := testutil.LocalArtifacts(t)

	host, err := env.DefaultScriptHost(
		broadcaster.NoopBroadcaster(),
		testlog.Logger(t, log.LevelInfo),
		common.Address{'D'},
		artifacts,
	)
	require.NoError(t, err)

	input := DeployAltDAInput{
		Salt:                     common.HexToHash("0x1234"),
		ProxyAdmin:               common.Address{'P'},
		ChallengeContractOwner:   common.Address{'O'},
		ChallengeWindow:          big.NewInt(100),
		ResolveWindow:            big.NewInt(200),
		BondSize:                 big.NewInt(300),
		ResolverRefundPercentage: big.NewInt(50), // must be < 100
	}

	output, err := DeployAltDA(host, input)
	require.NoError(t, err)

	require.NotEmpty(t, output.DataAvailabilityChallengeProxy)
	require.NotEmpty(t, output.DataAvailabilityChallengeImpl)
}
