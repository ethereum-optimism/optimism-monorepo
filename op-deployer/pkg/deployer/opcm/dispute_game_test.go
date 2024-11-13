package opcm

import (
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

func TestDeployDisputeGame(t *testing.T) {
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

	input := DeployDisputeGameInput{
		Release:                  "dev",
		StandardVersionsToml:     standardVersionsTOML,
		MipsVersion:              1,
		MinProposalSizeBytes:     standard.MinProposalSizeBytes,
		ChallengePeriodSeconds:   standard.ChallengePeriodSeconds,
		GameKind:                 "PermissionedDisputeGame",
		GameType:                 1,
		AbsolutePrestate:         common.Hash{'A'},
		MaxGameDepth:             standard.DisputeMaxGameDepth,
		SplitDepth:               standard.DisputeSplitDepth,
		ClockExtension:           standard.DisputeClockExtension,
		MaxClockDuration:         standard.DisputeMaxClockDuration,
		DelayedWethProxy:         common.Address{'D'},
		AnchorStateRegistryProxy: common.Address{'A'},
		L2ChainId:                69,
		Proposer:                 common.Address{'P'},
		Challenger:               common.Address{'C'},
	}

	output, err := DeployDisputeGame(host, input)
	require.NoError(t, err)

	require.NotEmpty(t, output.DisputeGameImpl)
	require.NotEmpty(t, output.MipsSingleton)
	require.NotEmpty(t, output.PreimageOracleSingleton)
}
