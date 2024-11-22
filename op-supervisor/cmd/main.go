package main

import (
	"context"
	"os"

	"github.com/ethereum-optimism/optimism/op-supervisor/config"
	"github.com/urfave/cli/v2"

	"github.com/ethereum/go-ethereum/log"

	opservice "github.com/ethereum-optimism/optimism/op-service"
	"github.com/ethereum-optimism/optimism/op-service/cliapp"
	"github.com/ethereum-optimism/optimism/op-service/ctxinterrupt"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	"github.com/ethereum-optimism/optimism/op-service/metrics/doc"
	"github.com/ethereum-optimism/optimism/op-supervisor/flags"
	"github.com/ethereum-optimism/optimism/op-supervisor/metrics"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor"
)

var (
	Version   = "v0.0.0"
	GitCommit = ""
	GitDate   = ""
)

func main() {
	ctx := ctxinterrupt.WithSignalWaiterMain(context.Background())
	err := run(ctx, os.Args, fromConfig)
	if err != nil {
		log.Crit("Application failed", "message", err)
	}
}

func run(ctx context.Context, args []string, fn supervisor.MainFn) error {
	oplog.SetupDefaults()

	app := cli.NewApp()
	app.Flags = cliapp.ProtectFlags(flags.Flags)
	app.Version = opservice.FormatVersion(Version, GitCommit, GitDate, "")
	app.Name = "op-supervisor"
	app.Usage = "op-supervisor monitors cross-L2 interop messaging"
	app.Description = "The op-supervisor monitors cross-L2 interop messaging by pre-fetching events and then resolving the cross-L2 dependencies to answer safety queries."
	app.Action = cliapp.LifecycleCmd(supervisor.Main(app.Version, fn))
	app.Commands = []*cli.Command{
		{
			Name:        "doc",
			Subcommands: doc.NewSubcommands(metrics.NewMetrics("default")),
		},
	}
	return app.RunContext(ctx, args)
}

func fromConfig(ctx context.Context, cfg *config.Config, logger log.Logger) (cliapp.Lifecycle, error) {
	return supervisor.SupervisorFromConfig(ctx, cfg, logger)
}
