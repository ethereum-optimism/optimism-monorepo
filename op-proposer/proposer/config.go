package proposer

import (
	"errors"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/ethereum-optimism/optimism/op-proposer/flags"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	opmetrics "github.com/ethereum-optimism/optimism/op-service/metrics"
	"github.com/ethereum-optimism/optimism/op-service/oppprof"
	oprpc "github.com/ethereum-optimism/optimism/op-service/rpc"
	"github.com/ethereum-optimism/optimism/op-service/txmgr"
)

// CLIConfig is a well typed config that is parsed from the CLI params.
// This also contains config options for auxiliary services.
// It is transformed into a `Config` before the L2 output submitter is started.
type CLIConfig struct {
	/* Required Params */

	// L1EthRpc is the HTTP provider URL for L1.
	L1EthRpc string

	// RollupRpc is the HTTP provider URL for the rollup node. A comma-separated list enables the active rollup provider.
	RollupRpc string

	// L2OOAddress is the L2OutputOracle contract address.
	L2OOAddress string

	// PollInterval is the delay between querying L2 for more transaction
	// and creating a new batch.
	PollInterval time.Duration

	// AllowNonFinalized can be set to true to propose outputs
	// for L2 blocks derived from non-finalized L1 data.
	AllowNonFinalized bool

	TxMgrConfig txmgr.CLIConfig

	RPCConfig oprpc.CLIConfig

	LogConfig oplog.CLIConfig

	MetricsConfig opmetrics.CLIConfig

	PprofConfig oppprof.CLIConfig

	// DGFAddress is the DisputeGameFactory contract address.
	DGFAddress string

	// ProposalInterval is the delay between submitting L2 output proposals when the DGFAddress is set.
	ProposalInterval time.Duration

	// DisputeGameType is the type of dispute game to create when submitting an output proposal.
	DisputeGameType uint32

	// ActiveSequencerCheckDuration is the duration between checks to determine the active sequencer endpoint.
	ActiveSequencerCheckDuration time.Duration

	// If Stopped is true, the proposer starts stopped and won't start proposing right away.
	// Proposing needs to be started via an admin RPC.
	Stopped bool

	// Whether to wait for the sequencer to sync to a recent block at startup.
	WaitNodeSync bool
}

func (c *CLIConfig) Check() error {
	if err := c.RPCConfig.Check(); err != nil {
		return err
	}
	if err := c.MetricsConfig.Check(); err != nil {
		return err
	}
	if err := c.PprofConfig.Check(); err != nil {
		return err
	}
	if err := c.TxMgrConfig.Check(); err != nil {
		return err
	}

	if c.DGFAddress == "" && c.L2OOAddress == "" {
		return errors.New("neither the `DisputeGameFactory` nor `L2OutputOracle` address was provided")
	}
	if c.DGFAddress != "" && c.L2OOAddress != "" {
		return errors.New("both the `DisputeGameFactory` and `L2OutputOracle` addresses were provided")
	}
	if c.DGFAddress != "" && c.ProposalInterval == 0 {
		return errors.New("the `DisputeGameFactory` address was provided but the `ProposalInterval` was not set")
	}
	if c.ProposalInterval != 0 && c.DGFAddress == "" {
		return errors.New("the `ProposalInterval` was provided but the `DisputeGameFactory` address was not set")
	}

	return nil
}

// NewConfig parses the Config from the provided flags or environment variables.
func NewConfig(ctx *cli.Context) *CLIConfig {
	return &CLIConfig{
		// Required Flags
		L1EthRpc:     ctx.String(flags.L1EthRpcFlag.Name),
		RollupRpc:    ctx.String(flags.RollupRpcFlag.Name),
		L2OOAddress:  ctx.String(flags.L2OOAddressFlag.Name),
		PollInterval: ctx.Duration(flags.PollIntervalFlag.Name),
		TxMgrConfig:  txmgr.ReadCLIConfig(ctx),
		// Optional Flags
		AllowNonFinalized:            ctx.Bool(flags.AllowNonFinalizedFlag.Name),
		RPCConfig:                    oprpc.ReadCLIConfig(ctx),
		LogConfig:                    oplog.ReadCLIConfig(ctx),
		MetricsConfig:                opmetrics.ReadCLIConfig(ctx),
		PprofConfig:                  oppprof.ReadCLIConfig(ctx),
		DGFAddress:                   ctx.String(flags.DisputeGameFactoryAddressFlag.Name),
		ProposalInterval:             ctx.Duration(flags.ProposalIntervalFlag.Name),
		DisputeGameType:              uint32(ctx.Uint(flags.DisputeGameTypeFlag.Name)),
		ActiveSequencerCheckDuration: ctx.Duration(flags.ActiveSequencerCheckDurationFlag.Name),
		Stopped:                      ctx.Bool(flags.StoppedFlag.Name),
		WaitNodeSync:                 ctx.Bool(flags.WaitNodeSyncFlag.Name),
	}
}
