package main

import (
	"context"
	"os"

	"github.com/ethereum-optimism/optimism/kurtosis-devnet-nat/pkg/nat"
	// "github.com/ethereum-optimism/optimism/kurtosis-devnet-nat/pkg/wallet"
	"github.com/ethereum-optimism/optimism/op-conductor/flags"
	opservice "github.com/ethereum-optimism/optimism/op-service"
	"github.com/ethereum-optimism/optimism/op-service/cliapp"
	"github.com/ethereum-optimism/optimism/op-service/ctxinterrupt"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	"github.com/ethereum/go-ethereum/log"
	"github.com/urfave/cli/v2"
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
	app.Name = "kurtosis-devnet-nat"
	app.Usage = "Kustosis Devnet Nat"
	app.Description = ""
	app.Action = cliapp.LifecycleCmd(NatMain)
	app.Commands = []*cli.Command{}

	ctx := ctxinterrupt.WithSignalWaiterMain(context.Background())
	err := app.RunContext(ctx, os.Args)
	if err != nil {
		log.Crit("Application failed", "message", err)
	}
}

func NatMain(ctx *cli.Context, closeApp context.CancelCauseFunc) (cliapp.Lifecycle, error) {
	logCfg := oplog.ReadCLIConfig(ctx)
	log := oplog.NewLogger(oplog.AppOut(ctx), logCfg)
	oplog.SetGlobalLogHandler(log.Handler())
	opservice.ValidateEnvVars(flags.EnvVarPrefix, flags.Flags, log)

	log.Info("Creating new network tester")
	c, err := nat.New(ctx.Context, log)
	if err != nil {
		log.Error("error creating network tester",
			"err", err)
		return nil, err
	}
	log.Info("nat created successfully")
	return c, nil
}
