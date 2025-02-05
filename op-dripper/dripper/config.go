package dripper

import (
	"errors"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/ethereum-optimism/optimism/op-dripper/flags"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	opmetrics "github.com/ethereum-optimism/optimism/op-service/metrics"
	"github.com/ethereum-optimism/optimism/op-service/oppprof"
	oprpc "github.com/ethereum-optimism/optimism/op-service/rpc"
	"github.com/ethereum-optimism/optimism/op-service/txmgr"
)

type CLIConfig struct {
	L1EthRpc       string
	DrippieAddress string
	PollInterval   time.Duration
	TxMgrConfig    txmgr.CLIConfig
	RPCConfig      oprpc.CLIConfig
	LogConfig      oplog.CLIConfig
	MetricsConfig  opmetrics.CLIConfig
	PprofConfig    oppprof.CLIConfig
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

	if c.DrippieAddress == "" {
		return errors.New("drippie address is required")
	}

	return nil
}

func NewConfig(ctx *cli.Context) *CLIConfig {
	return &CLIConfig{
		// Required Flags
		L1EthRpc:       ctx.String(flags.L1EthRpcFlag.Name),
		DrippieAddress: ctx.String(flags.DrippieAddressFlag.Name),
		PollInterval:   ctx.Duration(flags.PollIntervalFlag.Name),
		TxMgrConfig:    txmgr.ReadCLIConfig(ctx),

		// Optional Flags
		RPCConfig:     oprpc.ReadCLIConfig(ctx),
		LogConfig:     oplog.ReadCLIConfig(ctx),
		MetricsConfig: opmetrics.ReadCLIConfig(ctx),
		PprofConfig:   oppprof.ReadCLIConfig(ctx),
	}
}
