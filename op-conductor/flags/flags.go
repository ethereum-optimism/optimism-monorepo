package flags

import (
	"fmt"
	"time"

	"github.com/urfave/cli/v2"

	opservice "github.com/ethereum-optimism/optimism/op-service"
	opflags "github.com/ethereum-optimism/optimism/op-service/flags"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	opmetrics "github.com/ethereum-optimism/optimism/op-service/metrics"
	"github.com/ethereum-optimism/optimism/op-service/oppprof"
	oprpc "github.com/ethereum-optimism/optimism/op-service/rpc"
)

const EnvVarPrefix = "OP_CONDUCTOR"

var (
	ConsensusAddr = &cli.StringFlag{
		Name:    "consensus.addr",
		Usage:   "Address (excluding port) to listen for consensus connections.",
		EnvVars: opservice.PrefixEnvVar(EnvVarPrefix, "CONSENSUS_ADDR"),
		Value:   "127.0.0.1",
	}
	ConsensusPort = &cli.IntFlag{
		Name:    "consensus.port",
		Usage:   "Port to listen for consensus connections. May be 0 to let the system select a port.",
		EnvVars: opservice.PrefixEnvVar(EnvVarPrefix, "CONSENSUS_PORT"),
		Value:   50050,
	}
	AdvertisedFullAddr = &cli.StringFlag{
		Name:    "consensus.advertised",
		Usage:   "Full address (host and port) for other peers to contact the consensus server. Optional: if left empty, the local address is advertised.",
		EnvVars: opservice.PrefixEnvVar(EnvVarPrefix, "CONSENSUS_ADVERTISED"),
		Value:   "",
	}
	RaftBootstrap = &cli.BoolFlag{
		Name:    "raft.bootstrap",
		Usage:   "If this node should bootstrap a new raft cluster",
		EnvVars: opservice.PrefixEnvVar(EnvVarPrefix, "RAFT_BOOTSTRAP"),
		Value:   false,
	}
	RaftServerID = &cli.StringFlag{
		Name:    "raft.server.id",
		Usage:   "Unique ID for this server used by raft consensus",
		EnvVars: opservice.PrefixEnvVar(EnvVarPrefix, "RAFT_SERVER_ID"),
	}
	RaftStorageDir = &cli.StringFlag{
		Name:    "raft.storage.dir",
		Usage:   "Directory to store raft data",
		EnvVars: opservice.PrefixEnvVar(EnvVarPrefix, "RAFT_STORAGE_DIR"),
	}
	RaftSnapshotInterval = &cli.DurationFlag{
		Name:    "raft.snapshot-interval",
		Usage:   "The interval to check if a snapshot should be taken.",
		EnvVars: opservice.PrefixEnvVar(EnvVarPrefix, "RAFT_SNAPSHOT_INTERVAL"),
		Value:   120 * time.Second,
	}
	RaftSnapshotThreshold = &cli.Uint64Flag{
		Name:    "raft.snapshot-threshold",
		Usage:   "Number of logs to trigger a snapshot",
		EnvVars: opservice.PrefixEnvVar(EnvVarPrefix, "RAFT_SNAPSHOT_THRESHOLD"),
		Value:   8192,
	}
	RaftTrailingLogs = &cli.Uint64Flag{
		Name:    "raft.trailing-logs",
		Usage:   "Number of logs to keep after a snapshot",
		EnvVars: opservice.PrefixEnvVar(EnvVarPrefix, "RAFT_TRAILING_LOGS"),
		Value:   10240,
	}
	NodeRPC = &cli.StringFlag{
		Name:    "node.rpc",
		Usage:   "HTTP provider URL for op-node",
		EnvVars: opservice.PrefixEnvVar(EnvVarPrefix, "NODE_RPC"),
	}
	ExecutionRPC = &cli.StringFlag{
		Name:    "execution.rpc",
		Usage:   "HTTP provider URL for execution layer",
		EnvVars: opservice.PrefixEnvVar(EnvVarPrefix, "EXECUTION_RPC"),
	}
	HealthCheckInterval = &cli.Uint64Flag{
		Name:    "healthcheck.interval",
		Usage:   "Interval between health checks",
		EnvVars: opservice.PrefixEnvVar(EnvVarPrefix, "HEALTHCHECK_INTERVAL"),
	}
	HealthCheckUnsafeInterval = &cli.Uint64Flag{
		Name:    "healthcheck.unsafe-interval",
		Usage:   "Interval allowed between unsafe head and now measured in seconds",
		EnvVars: opservice.PrefixEnvVar(EnvVarPrefix, "HEALTHCHECK_UNSAFE_INTERVAL"),
	}
	HealthCheckSafeEnabled = &cli.BoolFlag{
		Name:    "healthcheck.safe-enabled",
		Usage:   "Whether to enable safe head progression checks",
		EnvVars: opservice.PrefixEnvVar(EnvVarPrefix, "HEALTHCHECK_SAFE_ENABLED"),
		Value:   false,
	}
	HealthCheckSafeInterval = &cli.Uint64Flag{
		Name:    "healthcheck.safe-interval",
		Usage:   "Interval between safe head progression measured in seconds",
		EnvVars: opservice.PrefixEnvVar(EnvVarPrefix, "HEALTHCHECK_SAFE_INTERVAL"),
		Value:   1200,
	}
	HealthCheckMinPeerCount = &cli.Uint64Flag{
		Name:    "healthcheck.min-peer-count",
		Usage:   "Minimum number of peers required to be considered healthy",
		EnvVars: opservice.PrefixEnvVar(EnvVarPrefix, "HEALTHCHECK_MIN_PEER_COUNT"),
	}
	Paused = &cli.BoolFlag{
		Name:    "paused",
		Usage:   "Whether the conductor is paused",
		EnvVars: opservice.PrefixEnvVar(EnvVarPrefix, "PAUSED"),
		Value:   false,
	}
	RPCEnableProxy = &cli.BoolFlag{
		Name:    "rpc.enable-proxy",
		Usage:   "Enable the RPC proxy to underlying sequencer services",
		EnvVars: opservice.PrefixEnvVar(EnvVarPrefix, "RPC_ENABLE_PROXY"),
		Value:   true,
	}
)

var requiredFlags = []cli.Flag{
	ConsensusAddr,
	ConsensusPort,
	RaftServerID,
	RaftStorageDir,
	NodeRPC,
	ExecutionRPC,
	HealthCheckInterval,
	HealthCheckUnsafeInterval,
	HealthCheckMinPeerCount,
}

var optionalFlags = []cli.Flag{
	AdvertisedFullAddr,
	Paused,
	RPCEnableProxy,
	RaftBootstrap,
	HealthCheckSafeEnabled,
	HealthCheckSafeInterval,
	RaftSnapshotInterval,
	RaftSnapshotThreshold,
	RaftTrailingLogs,
}

func init() {
	optionalFlags = append(optionalFlags, oprpc.CLIFlags(EnvVarPrefix)...)
	optionalFlags = append(optionalFlags, oplog.CLIFlags(EnvVarPrefix)...)
	optionalFlags = append(optionalFlags, opmetrics.CLIFlags(EnvVarPrefix)...)
	optionalFlags = append(optionalFlags, oppprof.CLIFlags(EnvVarPrefix)...)
	optionalFlags = append(optionalFlags, opflags.CLIFlags(EnvVarPrefix, "")...)

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
