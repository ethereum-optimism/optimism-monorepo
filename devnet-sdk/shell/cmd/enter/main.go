package main

import (
	"fmt"
	"os"
	"os/exec"

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

	if motd := chainEnv.Motd; motd != "" {
		fmt.Println(motd)
	}

	// Get current environment and append chain-specific vars
	env := os.Environ()
	for key, value := range chainEnv.EnvVars {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}

	// Get current shell
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}

	// Execute new shell
	cmd := exec.Command(shell)
	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error executing shell: %w", err)
	}

	return nil
}

func main() {
	app := &cli.App{
		Name:  "enter",
		Usage: "Enter a shell with devnet environment variables set",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "devnet",
				Usage:    "Path to devnet JSON file",
				EnvVars:  []string{env.EnvFileVar},
				Required: true,
			},
			&cli.StringFlag{
				Name:     "chain",
				Usage:    "Name of the chain to connect to",
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
