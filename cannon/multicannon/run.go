package main

import (
	"fmt"
	"os"

	"github.com/urfave/cli/v2"

	"github.com/ethereum-optimism/optimism/cannon/mipsevm/versions"
)

func Run(ctx *cli.Context) error {
	if len(os.Args) == 3 && os.Args[2] == "--help" {
		if err := list(); err != nil {
			return err
		}
		fmt.Println("use `--input <valid input file> --help` to get more detailed help")
		return nil
	}

	inputPath, err := parsePathFlag(os.Args[1:], "--input")
	if err != nil {
		return err
	}
	version, err := versions.DetectVersion(inputPath)
	if err != nil {
		return err
	}
	return ExecuteCannon(ctx.Context, os.Args[1:], version)
}

var RunCommand = &cli.Command{
	Name:            "run",
	Usage:           "Run VM step(s) and generate proof data to replicate onchain.",
	Description:     "Run VM step(s) and generate proof data to replicate onchain. See flags to match when to output a proof, a snapshot, or to stop early.",
	Action:          Run,
	SkipFlagParsing: true,
}
