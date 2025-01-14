package main

import (
	"context"
	"os"

	"github.com/ethereum-optimism/optimism/op-service/ctxinterrupt"

	opservice "github.com/ethereum-optimism/optimism/op-service"
	"github.com/urfave/cli/v2"

	"github.com/ethereum-optimism/optimism/op-dripper/dripper"
	"github.com/ethereum-optimism/optimism/op-dripper/flags"
	"github.com/ethereum-optimism/optimism/op-dripper/metrics"
	"github.com/ethereum-optimism/optimism/op-service/cliapp"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	"github.com/ethereum-optimism/optimism/op-service/metrics/doc"
	"github.com/ethereum/go-ethereum/log"
)

var (
	Version   = "v0.0.0"
	GitCommit = ""
	GitDate   = ""
)

func main() {
	oplog.SetupDefaults()

	app := cli.NewApp()
	app.Flags = cliapp.ProtectFlags(flags.Flags)
	app.Version = opservice.FormatVersion(Version, GitCommit, GitDate, "")
	app.Name = "op-dripper"
	app.Usage = "Drippie Executor"
	app.Description = "Service for executing Drippie drips"
	app.Action = cliapp.LifecycleCmd(dripper.Main(Version))
	app.Commands = []*cli.Command{
		{
			Name:        "doc",
			Subcommands: doc.NewSubcommands(metrics.NewMetrics("default")),
		},
	}

	ctx := ctxinterrupt.WithSignalWaiterMain(context.Background())
	err := app.RunContext(ctx, os.Args)
	if err != nil {
		log.Crit("Application failed", "message", err)
	}
}
