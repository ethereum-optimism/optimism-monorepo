package flags

import (
	"time"

	"github.com/urfave/cli/v2"

	opservice "github.com/ethereum-optimism/optimism/op-service"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	opmetrics "github.com/ethereum-optimism/optimism/op-service/metrics"
	"github.com/ethereum-optimism/optimism/op-service/oppprof"
	oprpc "github.com/ethereum-optimism/optimism/op-service/rpc"
	"github.com/ethereum-optimism/optimism/op-service/txmgr"
)

const EnvVarPrefix = "OP_PROPOSER"

func prefixEnvVars(name string) []string {
	return opservice.PrefixEnvVar(EnvVarPrefix, name)
}

var (
	// Required Flags
	L1EthRpcFlag = &cli.StringFlag{
		Name:     "l1-eth-rpc",
		Usage:    "HTTP provider URL for L1",
		EnvVars:  prefixEnvVars("L1_ETH_RPC"),
		Required: true,
	}
	RollupRpcFlag = &cli.StringFlag{
		Name:     "rollup-rpc",
		Usage:    "HTTP provider URL for the rollup node. A comma-separated list enables the active rollup provider.",
		EnvVars:  prefixEnvVars("ROLLUP_RPC"),
		Required: true,
	}

	// Optional flags
	L2OOAddressFlag = &cli.StringFlag{
		Name:    "l2oo-address",
		Usage:   "Address of the L2OutputOracle contract",
		EnvVars: prefixEnvVars("L2OO_ADDRESS"),
	}
	PollIntervalFlag = &cli.DurationFlag{
		Name:    "poll-interval",
		Usage:   "How frequently to poll L2 for new blocks (legacy L2OO)",
		Value:   12 * time.Second,
		EnvVars: prefixEnvVars("POLL_INTERVAL"),
	}
	AllowNonFinalizedFlag = &cli.BoolFlag{
		Name:    "allow-non-finalized",
		Usage:   "Allow the proposer to submit proposals for L2 blocks derived from non-finalized L1 blocks.",
		EnvVars: prefixEnvVars("ALLOW_NON_FINALIZED"),
	}
	DisputeGameFactoryAddressFlag = &cli.StringFlag{
		Name:    "game-factory-address",
		Usage:   "Address of the DisputeGameFactory contract",
		EnvVars: prefixEnvVars("GAME_FACTORY_ADDRESS"),
	}
	ProposalIntervalFlag = &cli.DurationFlag{
		Name:    "proposal-interval",
		Usage:   "Interval between submitting L2 output proposals when the dispute game factory address is set",
		EnvVars: prefixEnvVars("PROPOSAL_INTERVAL"),
	}
	DisputeGameTypeFlag = &cli.UintFlag{
		Name:    "game-type",
		Usage:   "Dispute game type to create via the configured DisputeGameFactory",
		Value:   0,
		EnvVars: prefixEnvVars("GAME_TYPE"),
	}
	ActiveSequencerCheckDurationFlag = &cli.DurationFlag{
		Name:    "active-sequencer-check-duration",
		Usage:   "The duration between checks to determine the active sequencer endpoint.",
		Value:   2 * time.Minute,
		EnvVars: prefixEnvVars("ACTIVE_SEQUENCER_CHECK_DURATION"),
	}
	WaitNodeSyncFlag = &cli.BoolFlag{
		Name: "wait-node-sync",
		Usage: "Indicates if, during startup, the proposer should wait for the rollup node to sync to " +
			"the current L1 tip before proceeding with its driver loop.",
		Value:   false,
		EnvVars: prefixEnvVars("WAIT_NODE_SYNC"),
	}
	// Legacy Flags
	L2OutputHDPathFlag = txmgr.L2OutputHDPathFlag
)

// Flags contains the list of configuration options available to the binary.
var Flags = []cli.Flag{
	L1EthRpcFlag,
	RollupRpcFlag,
	L2OOAddressFlag,
	PollIntervalFlag,
	AllowNonFinalizedFlag,
	L2OutputHDPathFlag,
	DisputeGameFactoryAddressFlag,
	ProposalIntervalFlag,
	DisputeGameTypeFlag,
	ActiveSequencerCheckDurationFlag,
	WaitNodeSyncFlag,
}

func init() {
	Flags = append(Flags, oprpc.CLIFlags(EnvVarPrefix)...)
	Flags = append(Flags, oplog.CLIFlags(EnvVarPrefix)...)
	Flags = append(Flags, opmetrics.CLIFlags(EnvVarPrefix)...)
	Flags = append(Flags, oppprof.CLIFlags(EnvVarPrefix)...)
	Flags = append(Flags, txmgr.CLIFlags(EnvVarPrefix)...)
}
