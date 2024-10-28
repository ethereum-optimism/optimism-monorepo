package deployer

import (
	"fmt"
	"os"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/state"

	op_service "github.com/ethereum-optimism/optimism/op-service"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	"github.com/urfave/cli/v2"
)

const (
	EnvVarPrefix               = "DEPLOYER"
	L1RPCURLFlagName           = "l1-rpc-url"
	L1ChainIDFlagName          = "l1-chain-id"
	L2ChainIDsFlagName         = "l2-chain-ids"
	WorkdirFlagName            = "workdir"
	OutdirFlagName             = "outdir"
	PrivateKeyFlagName         = "private-key"
	DeploymentStrategyFlagName = "deployment-strategy"
)

var (
	L1RPCURLFlag = &cli.StringFlag{
		Name: L1RPCURLFlagName,
		Usage: "RPC URL for the L1 chain. Must be set for live chains. " +
			"Can be blank for chains deploying to local allocs files.",
		EnvVars: []string{
			"L1_RPC_URL",
		},
	}
	L1ChainIDFlag = &cli.Uint64Flag{
		Name:    L1ChainIDFlagName,
		Usage:   "Chain ID of the L1 chain.",
		EnvVars: PrefixEnvVar("L1_CHAIN_ID"),
		Value:   900,
	}
	L2ChainIDsFlag = &cli.StringFlag{
		Name:    L2ChainIDsFlagName,
		Usage:   "Comma-separated list of L2 chain IDs to deploy.",
		EnvVars: PrefixEnvVar("L2_CHAIN_IDS"),
	}
	WorkdirFlag = &cli.StringFlag{
		Name:    WorkdirFlagName,
		Usage:   "Directory storing intent and stage. Defaults to the current directory.",
		EnvVars: PrefixEnvVar("WORKDIR"),
		Value:   cwd(),
		Aliases: []string{
			OutdirFlagName,
		},
	}
	PrivateKeyFlag = &cli.StringFlag{
		Name:    PrivateKeyFlagName,
		Usage:   "Private key of the deployer account.",
		EnvVars: PrefixEnvVar("PRIVATE_KEY"),
	}
	DeploymentStrategyFlag = &cli.StringFlag{
		Name:    DeploymentStrategyFlagName,
		Usage:   fmt.Sprintf("Deployment strategy to use. Options: %s, %s", state.DeploymentStrategyLive, state.DeploymentStrategyGenesis),
		EnvVars: PrefixEnvVar("DEPLOYMENT_STRATEGY"),
		Value:   string(state.DeploymentStrategyLive),
	}
)

var GlobalFlags = append([]cli.Flag{}, oplog.CLIFlags(EnvVarPrefix)...)

var InitFlags = []cli.Flag{
	L1ChainIDFlag,
	L2ChainIDsFlag,
	WorkdirFlag,
	DeploymentStrategyFlag,
}

var ApplyFlags = []cli.Flag{
	L1RPCURLFlag,
	WorkdirFlag,
	PrivateKeyFlag,
}

func PrefixEnvVar(name string) []string {
	return op_service.PrefixEnvVar(EnvVarPrefix, name)
}

func cwd() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	return dir
}
