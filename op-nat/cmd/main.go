package main

import (
	"context"
	"fmt"
	"os"

	"github.com/ethereum/go-ethereum/log"
	"github.com/urfave/cli/v2"

	nat "github.com/ethereum-optimism/optimism/op-nat"
	"github.com/ethereum-optimism/optimism/op-nat/flags"
	"github.com/ethereum-optimism/optimism/op-nat/validators/gates"
	opservice "github.com/ethereum-optimism/optimism/op-service"
	"github.com/ethereum-optimism/optimism/op-service/cliapp"
	"github.com/ethereum-optimism/optimism/op-service/ctxinterrupt"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
)

var (
	Version   = "v0.0.1"
	GitCommit = ""
	GitDate   = ""
)

func main() {
	oplog.SetupDefaults()

	app := cli.NewApp()
	app.Flags = cliapp.ProtectFlags(flags.Flags)
	app.Version = opservice.FormatVersion(Version, GitCommit, GitDate, "")
	app.Name = "op-nat"
	app.Usage = "Optimism Network Acceptance Tester Service"
	app.Description = "op-nat tests networks"
	app.Action = cliapp.LifecycleCmd(run)

	ctx := ctxinterrupt.WithSignalWaiterMain(context.Background())
	err := app.RunContext(ctx, os.Args)
	if err != nil {
		log.Crit("Application failed", "message", err)
	}
}

func run(ctx *cli.Context, closeApp context.CancelCauseFunc) (cliapp.Lifecycle, error) {
	logCfg := oplog.ReadCLIConfig(ctx)
	log := oplog.NewLogger(oplog.AppOut(ctx), logCfg)
	oplog.SetGlobalLogHandler(log.Handler())
	opservice.ValidateEnvVars(flags.EnvVarPrefix, flags.Flags, log)

	// TODO: map validators from flags
	var validators = []nat.Validator{
		&gates.Alphanet,
	}

	cfg, err := nat.NewConfig(ctx, validators)
	if err != nil {
		return nil, fmt.Errorf("failed to create config: %w", err)
	}

	log.Info("Config", "config", cfg)

	c, err := nat.New(ctx.Context, cfg, log, Version)
	if err != nil {
		return nil, fmt.Errorf("failed to create nat: %w", err)
	}

	return c, nil
}
