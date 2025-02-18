package main

import (
	"fmt"
	"os"

	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	"github.com/ethereum-optimism/optimism/op-validator/pkg/service"
	"github.com/urfave/cli/v2"
)

const EnvVarPrefix = "OP_VALIDATOR"

var (
	GitCommit = ""
	GitDate   = ""
	Version   = ""
)

func main() {
	app := cli.NewApp()
	app.Version = Version
	app.Name = "op-validator"
	app.Usage = "Optimism Validator Service"
	app.Description = "CLI to validate Optimism L2 deployments"
	app.Flags = oplog.CLIFlags(EnvVarPrefix)
	app.Commands = []*cli.Command{
		{
			Name:  "validate",
			Usage: "Run validation for a specific version",
			Subcommands: []*cli.Command{
				{
					Name:  "v1.8.0",
					Usage: "Run validation for v1.8.0",
					Flags: append(service.ValidateFlags, oplog.CLIFlags(EnvVarPrefix)...),
					Action: func(cliCtx *cli.Context) error {
						return service.ValidateCmd(cliCtx, "v1.8.0")
					},
				},
				{
					Name:  "v2.0.0",
					Usage: "Run validation for v2.0.0",
					Flags: append(service.ValidateFlags, oplog.CLIFlags(EnvVarPrefix)...),
					Action: func(cliCtx *cli.Context) error {
						return service.ValidateCmd(cliCtx, "v2.0.0")
					},
				},
			},
		},
	}
	app.Writer = os.Stdout
	app.ErrWriter = os.Stderr

	err := app.Run(os.Args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Application failed: %v\n", err)
		os.Exit(1)
	}
}
