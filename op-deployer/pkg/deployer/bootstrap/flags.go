package bootstrap

import (
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer"
	"github.com/ethereum-optimism/optimism/op-service/cliapp"
	"github.com/urfave/cli/v2"
)

const (
	ArtifactsLocatorFlagName = "artifacts-locator"
)

var (
	ArtifactsLocatorFlag = &cli.StringFlag{
		Name:    ArtifactsLocatorFlagName,
		Usage:   "Locator for artifacts.",
		EnvVars: deployer.PrefixEnvVar("ARTIFACTS_LOCATOR"),
	}
)

var OPCMFlags = []cli.Flag{
	deployer.L1RPCURLFlag,
	deployer.PrivateKeyFlag,
	ArtifactsLocatorFlag,
}

var Commands = []*cli.Command{
	{
		Name:   "opcm",
		Usage:  "Bootstrap an instance of OPCM.",
		Flags:  cliapp.ProtectFlags(OPCMFlags),
		Action: OPCMCLI,
	},
}
