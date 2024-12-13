package main

import (
	"fmt"
	"os"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/bootstrap"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/inspect"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/version"

	opservice "github.com/ethereum-optimism/optimism/op-service"

	"github.com/ethereum-optimism/optimism/op-service/cliapp"
	"github.com/urfave/cli/v2"
)

var (
	GitCommit = ""
	GitDate   = ""
)

// VersionWithMeta holds the textual version string including the metadata.
var VersionWithMeta = opservice.FormatVersion(version.Version, GitCommit, GitDate, version.Meta)

func main() {
	app := cli.NewApp()
	app.Version = VersionWithMeta
	app.Name = "op-deployer"
	app.Usage = "Tool to configure and deploy OP Chains."
	app.Flags = cliapp.ProtectFlags(deployer.GlobalFlags)
	app.Commands = []*cli.Command{
		{
			Name:   "init",
			Usage:  "initializes a chain intent and state file",
			Flags:  cliapp.ProtectFlags(deployer.InitFlags),
			Action: deployer.InitCLI(),
		},
		{
			Name:   "apply",
			Usage:  "applies a chain intent to the chain",
			Flags:  cliapp.ProtectFlags(deployer.ApplyFlags),
			Action: deployer.ApplyCLI(),
		},
		{
			Name:        "bootstrap",
			Usage:       "bootstraps global contract instances",
			Subcommands: bootstrap.Commands,
		},
		{
			Name:        "inspect",
			Usage:       "inspects the state of a deployment",
			Subcommands: inspect.Commands,
		},
	}
	app.Writer = os.Stdout
	app.ErrWriter = os.Stderr
	err := app.Run(os.Args)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Application failed: %v\n", err)
		os.Exit(1)
	}
}
