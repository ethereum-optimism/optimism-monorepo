package main

import (
	"fmt"
	"os"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/manage"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/bootstrap"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/inspect"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/version"

	opservice "github.com/ethereum-optimism/optimism/op-service"

	"github.com/ethereum-optimism/optimism/op-service/cliapp"
	"github.com/urfave/cli/v2"
)

// GitCommit contains the current git commit hash
var GitCommit = ""
// GitDate contains the build date
var GitDate = ""

// VersionWithMeta holds the textual version string including the metadata
var VersionWithMeta = opservice.FormatVersion(version.Version, GitCommit, GitDate, version.Meta)

func main() {
	app := cli.NewApp()
	app.Version = VersionWithMeta
	app.Name = "op-deployer"
	app.Usage = "Tool to configure and deploy OP Chains."
	app.Description = `A comprehensive tool for deploying and managing Optimism chains. 
This application provides functionality for initialization, deployment, 
bootstrapping, and management of OP Chain instances.`
	app.Flags = cliapp.ProtectFlags(deployer.GlobalFlags)
	app.Commands = []*cli.Command{
		{
			Name:      "init",
			Usage:     "initializes a chain intent and state file",
			Category:  "Setup",
			Flags:     cliapp.ProtectFlags(deployer.InitFlags),
			Action:    deployer.InitCLI(),
		},
		{
			Name:      "apply",
			Usage:     "applies a chain intent to the chain",
			Category:  "Deployment",
			Flags:     cliapp.ProtectFlags(deployer.ApplyFlags),
			Action:    deployer.ApplyCLI(),
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
		{
			Name:        "manage",
			Usage:       "performs individual operations on a chain",
			Subcommands: manage.Commands,
		},
	}
	app.Writer = os.Stdout
	app.ErrWriter = os.Stderr
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Application failed: %v\n", err)
		os.Exit(1)
	}
}
