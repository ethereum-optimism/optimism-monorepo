package flags

import (
	"fmt"

	"github.com/urfave/cli/v2"

	opservice "github.com/ethereum-optimism/optimism/op-service"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	opmetrics "github.com/ethereum-optimism/optimism/op-service/metrics"
	"github.com/ethereum-optimism/optimism/op-service/oppprof"
	oprpc "github.com/ethereum-optimism/optimism/op-service/rpc"
	"github.com/ethereum-optimism/optimism/op-supervisor/config"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/syncnode"
)

const EnvVarPrefix = "OP_SUPERVISOR"

func prefixEnvVars(name string) []string {
	return opservice.PrefixEnvVar(EnvVarPrefix, name)
}

var (
	L1RPCFlag = &cli.StringFlag{
		Name:    "l1-rpc",
		Usage:   "L1 RPC source.",
		EnvVars: prefixEnvVars("L1_RPC"),
	}
	L2ConsensusNodesFlag = &cli.StringSliceFlag{
		Name:    "l2-consensus.nodes",
		Usage:   "L2 Consensus rollup node RPC addresses (with auth).",
		EnvVars: prefixEnvVars("L2_CONSENSUS_NODES"),
	}
	L2ConsensusJWTSecret = &cli.StringSliceFlag{
		Name: "l2-consensus.jwt-secret",
		Usage: "Path to JWT secret key. Keys are 32 bytes, hex encoded in a file. " +
			"If multiple paths are specified, secrets are assumed to match l2-consensus-nodes order.",
		EnvVars:   prefixEnvVars("L2_CONSENSUS_JWT_SECRET"),
		Value:     cli.NewStringSlice(),
		TakesFile: true,
	}
	DataDirFlag = &cli.PathFlag{
		Name:    "datadir",
		Usage:   "Directory to store data generated as part of responding to games",
		EnvVars: prefixEnvVars("DATADIR"),
	}
	DataDirSyncEndpointFlag = &cli.PathFlag{
		Name:    "datadir.sync-endpoint",
		Usage:   "op-supervisor endpoint to sync databases from",
		EnvVars: prefixEnvVars("DATADIR_SYNC_ENDPOINT"),
	}
	DependencySetFlag = &cli.PathFlag{
		Name:      "dependency-set",
		Usage:     "Dependency-set configuration, point at JSON file.",
		EnvVars:   prefixEnvVars("DEPENDENCY_SET"),
		TakesFile: true,
	}
	MockRunFlag = &cli.BoolFlag{
		Name:    "mock-run",
		Usage:   "Mock run, no actual backend used, just presenting the service",
		EnvVars: prefixEnvVars("MOCK_RUN"),
		Hidden:  true, // this is for testing only
	}
)

var requiredFlags = []cli.Flag{
	L1RPCFlag,
	L2ConsensusNodesFlag,
	L2ConsensusJWTSecret,
	DataDirFlag,
	DependencySetFlag,
}

var optionalFlags = []cli.Flag{
	MockRunFlag,
	DataDirSyncEndpointFlag,
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
		Version:             version,
		LogConfig:           oplog.ReadCLIConfig(ctx),
		MetricsConfig:       opmetrics.ReadCLIConfig(ctx),
		PprofConfig:         oppprof.ReadCLIConfig(ctx),
		RPC:                 oprpc.ReadCLIConfig(ctx),
		DependencySetSource: &depset.JsonDependencySetLoader{Path: ctx.Path(DependencySetFlag.Name)},
		MockRun:             ctx.Bool(MockRunFlag.Name),
		L1RPC:               ctx.String(L1RPCFlag.Name),
		SyncSources:         syncSourceSetups(ctx),
		Datadir:             ctx.Path(DataDirFlag.Name),
		DatadirSyncEndpoint: ctx.Path(DataDirSyncEndpointFlag.Name),
	}
}

// syncSourceSetups creates a sync source collection, from CLI arguments.
// These sources can share JWT secret configuration.
func syncSourceSetups(ctx *cli.Context) syncnode.SyncNodeCollection {
	return &syncnode.CLISyncNodes{
		Endpoints:      filterEmpty(ctx.StringSlice(L2ConsensusNodesFlag.Name)),
		JWTSecretPaths: filterEmpty(ctx.StringSlice(L2ConsensusJWTSecret.Name)),
	}
}

// filterEmpty cleans empty entries from a string-slice flag,
// which has the potential to have empty strings.
func filterEmpty(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}
