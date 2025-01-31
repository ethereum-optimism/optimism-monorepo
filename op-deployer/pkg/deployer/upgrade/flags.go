package upgrade

import (
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer"
	v200 "github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/upgrade/v2_0_0"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	"github.com/urfave/cli/v2"
)

var (
	ConfigFlag = &cli.StringFlag{
		Name:  "config",
		Usage: "path to the config file",
	}
	OverrideArtifactsURLFlag = &cli.StringFlag{
		Name:  "override-artifacts-url",
		Usage: "override the artifacts URL",
	}
)

var Commands = cli.Commands{
	&cli.Command{
		Name:  "v2.0.0",
		Usage: "upgrades a chain to version v2.0.0",
		Flags: append([]cli.Flag{
			deployer.L1RPCURLFlag,
			deployer.DeploymentTargetFlag,
			deployer.PrivateKeyFlag,
			ConfigFlag,
			OverrideArtifactsURLFlag,
		}, oplog.CLIFlags(deployer.EnvVarPrefix)...),
		Action: UpgradeCLI(v200.DefaultUpgrader),
	},
}
