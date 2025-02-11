package flags

import (
	"fmt"

	"github.com/urfave/cli/v2"

	opservice "github.com/ethereum-optimism/optimism/op-service"
	"github.com/ethereum-optimism/optimism/op-service/endpoint"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	opmetrics "github.com/ethereum-optimism/optimism/op-service/metrics"
	"github.com/ethereum-optimism/optimism/op-service/oppprof"
	oprpc "github.com/ethereum-optimism/optimism/op-service/rpc"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/config"
)

const EnvVarPrefix = "OP_TEST_SEQUENCER"

func prefixEnvVars(name string) []string {
	return opservice.PrefixEnvVar(EnvVarPrefix, name)
}

var (
	L2CLEndpointFlag = &cli.StringFlag{
		Name:    "l2.cl",
		Usage:   "Connect to L2 rollup consensus layer RPC",
		EnvVars: prefixEnvVars("L2_CL"),
		Value:   config.DefaultL2CL,
	}
	L2ELEndpointFlag = &cli.StringFlag{
		Name:    "l2.el",
		Usage:   "Connect to L2 execution layer RPC for state data",
		EnvVars: prefixEnvVars("L2_EL"),
		Value:   config.DefaultL2EL,
	}
	L1ELEndpointFlag = &cli.StringFlag{
		Name:    "l1.el",
		Usage:   "Connect to L1 execution layer for block data",
		EnvVars: prefixEnvVars("L1_EL"),
		Value:   config.DefaultL1EL,
	}
	RPCJWTSecretFlag = &cli.StringFlag{
		Name:      "rpc.jwt-secret",
		Usage:     "Path to JWT secret key for sequencer admin RPC.",
		EnvVars:   prefixEnvVars("RPC_JWT_SECRET"),
		TakesFile: true,
	}
	MockRunFlag = &cli.BoolFlag{
		Name:    "mock-run",
		Usage:   "Mock run, no actual backend used, just presenting the service",
		EnvVars: prefixEnvVars("MOCK_RUN"),
		Hidden:  true, // this is for testing only
	}
)

var requiredFlags = []cli.Flag{}

var optionalFlags = []cli.Flag{
	L2CLEndpointFlag,
	L2ELEndpointFlag,
	L1ELEndpointFlag,
	RPCJWTSecretFlag,
	MockRunFlag,
}

func init() {
	optionalFlags = append(optionalFlags, oprpc.CLIFlags(EnvVarPrefix)...)
	optionalFlags = append(optionalFlags, oplog.CLIFlags(EnvVarPrefix)...)
	optionalFlags = append(optionalFlags, opmetrics.CLIFlags(EnvVarPrefix)...)
	optionalFlags = append(optionalFlags, oppprof.CLIFlags(EnvVarPrefix)...)

	Flags = append(Flags, requiredFlags...)
	Flags = append(Flags, optionalFlags...)
}

// Flags contains the list of configuration options available to the binary.
var Flags []cli.Flag

func CheckRequired(ctx *cli.Context) error {
	for _, f := range requiredFlags {
		if !ctx.IsSet(f.Names()[0]) {
			return fmt.Errorf("flag %s is required", f.Names()[0])
		}
	}
	return nil
}

func ConfigFromCLI(ctx *cli.Context, version string) *config.Config {
	return &config.Config{
		Version:       version,
		LogConfig:     oplog.ReadCLIConfig(ctx),
		MetricsConfig: opmetrics.ReadCLIConfig(ctx),
		PprofConfig:   oppprof.ReadCLIConfig(ctx),
		RPC:           oprpc.ReadCLIConfig(ctx),
		JWTSecretPath: ctx.Path(RPCJWTSecretFlag.Name),
		L2CL:          endpoint.URL(ctx.String(L2CLEndpointFlag.Name)),
		L2EL:          endpoint.URL(ctx.String(L2ELEndpointFlag.Name)),
		L1EL:          endpoint.URL(ctx.String(L1ELEndpointFlag.Name)),
		MockRun:       ctx.Bool(MockRunFlag.Name),
	}
}
