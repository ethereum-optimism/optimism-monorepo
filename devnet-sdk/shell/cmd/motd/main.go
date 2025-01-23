package main

import (
	"fmt"
	"os"

	"github.com/ethereum-optimism/optimism/devnet-sdk/shell/env"
	"github.com/urfave/cli/v2"
)

func run(ctx *cli.Context) error {
	devnetFile := ctx.String("devnet")
	chainName := ctx.String("chain")

	devnetEnv, err := env.LoadDevnetEnv(devnetFile)
	if err != nil {
		return err
	}

	chain, err := devnetEnv.GetChain(chainName)
	if err != nil {
		return err
	}

	chainEnv, err := chain.GetEnv()
	if err != nil {
		return err
	}

	fmt.Println(chainEnv.Motd)
	return nil
}

func main() {
	app := &cli.App{
		Name:  "motd",
		Usage: "Display the Message of the Day for a chain environment",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "devnet",
				Usage:    "Path to devnet JSON file",
				EnvVars:  []string{env.EnvFileVar},
				Required: true,
			},
			&cli.StringFlag{
				Name:     "chain",
				Usage:    "Name of the chain to get MOTD for",
				EnvVars:  []string{env.ChainNameVar},
				Required: true,
			},
		},
		Action: run,
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
