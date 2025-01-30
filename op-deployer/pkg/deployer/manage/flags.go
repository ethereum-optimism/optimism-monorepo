package manage

import (
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/bootstrap"
	"github.com/ethereum-optimism/optimism/op-service/cliapp"
	"github.com/urfave/cli/v2"
)

const (
	ChainIDFlagName      = "chain-id"
	SystemConfigFlagName = "system-config"
	RemoveFlagName       = "remove"
)

var (
	ChainIDFlagManageDependencies = &cli.StringFlag{
		Name:     ChainIDFlagName,
		Usage:    "The chain ID to add or remove.",
		EnvVars:  deployer.PrefixEnvVar("CHAIN_ID"),
		Required: true,
	}
	SystemConfigFlagManageDependencies = &cli.StringFlag{
		Name:     SystemConfigFlagName,
		Usage:    "The system config of the chain whose dependencies are being managed.",
		EnvVars:  deployer.PrefixEnvVar("SYSTEM_CONFIG"),
		Required: true,
	}
	RemoveFlagManageDependencies = &cli.BoolFlag{
		Name:    RemoveFlagName,
		Usage:   "Remove the dependency instead of adding it.",
		EnvVars: deployer.PrefixEnvVar("REMOVE"),
	}
)

var Commands = []*cli.Command{
	{
		Name:  "dependencies",
		Usage: "Manage dependencies for a chain's interop set.",
		Flags: cliapp.ProtectFlags([]cli.Flag{
			deployer.L1RPCURLFlag,
			deployer.PrivateKeyFlag,
			bootstrap.ArtifactsLocatorFlag,
			ChainIDFlagManageDependencies,
			SystemConfigFlagManageDependencies,
			RemoveFlagManageDependencies,
		}),
		Action: DependenciesCLI,
	},
}
