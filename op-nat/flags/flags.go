package flags

import (
	"fmt"

	"github.com/urfave/cli/v2"

	opservice "github.com/ethereum-optimism/optimism/op-service"
	opflags "github.com/ethereum-optimism/optimism/op-service/flags"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	opmetrics "github.com/ethereum-optimism/optimism/op-service/metrics"
	oprpc "github.com/ethereum-optimism/optimism/op-service/rpc"
)

const EnvVarPrefix = "OP_NAT"

var (
	KurtosisDevnetManifest = &cli.StringFlag{
		Name:    "kurtosis.devnet.manifest",
		Value:   "",
		EnvVars: opservice.PrefixEnvVar(EnvVarPrefix, "KURTOSIS_DEVNET_MANIFEST"),
		Usage:   "Path to the kurtosis-devnet manifest",
	}
	ValidatorFilter = &cli.StringFlag{
		Name:    "validator.filter",
		Value:   "",
		EnvVars: opservice.PrefixEnvVar(EnvVarPrefix, "VALIDATOR_FILTER"),
		Usage:   "Name of the specific suite to test",
	}
)

var requiredFlags = []cli.Flag{
	KurtosisDevnetManifest,
}

var optionalFlags = []cli.Flag{
	ValidatorFilter,
}

func init() {
	optionalFlags = append(optionalFlags, oprpc.CLIFlags(EnvVarPrefix)...)
	optionalFlags = append(optionalFlags, oplog.CLIFlags(EnvVarPrefix)...)
	optionalFlags = append(optionalFlags, opmetrics.CLIFlags(EnvVarPrefix)...)
	// optionalFlags = append(optionalFlags, oppprof.CLIFlags(EnvVarPrefix)...)
	// optionalFlags = append(optionalFlags, opflags.CLIFlags(EnvVarPrefix, "")...)

	Flags = append(requiredFlags, optionalFlags...)
}

var Flags []cli.Flag

func CheckRequired(ctx *cli.Context) error {
	for _, f := range requiredFlags {
		if !ctx.IsSet(f.Names()[0]) {
			return fmt.Errorf("flag %s is required", f.Names()[0])
		}
	}
	return opflags.CheckRequiredXor(ctx)
}
