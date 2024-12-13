package inspect

import (
	"fmt"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer"

	op_service "github.com/ethereum-optimism/optimism/op-service"

	"github.com/ethereum/go-ethereum/common"
	"github.com/urfave/cli/v2"
)

const (
	OutfileFlagName = "outfile"
)

var (
	FlagOutfile = &cli.StringFlag{
		Name:  OutfileFlagName,
		Usage: "output file. set to - to use stdout",
		Value: "-",
	}
)

var Flags = []cli.Flag{
	deployer.WorkdirFlag,
	FlagOutfile,
}

var Commands = []*cli.Command{
	{
		Name:      "l1",
		Usage:     "outputs all L1 contract addresses for an L2 chain",
		Args:      true,
		ArgsUsage: "<l2-chain-id>",
		Action:    L1CLI,
		Flags:     Flags,
	},
	{
		Name:      "genesis",
		Usage:     "outputs the genesis for an L2 chain",
		Args:      true,
		ArgsUsage: "<l2-chain-id>",
		Action:    GenesisCLI,
		Flags:     Flags,
	},
	{
		Name:      "rollup",
		Usage:     "outputs the rollup config for an L2 chain",
		Args:      true,
		ArgsUsage: "<l2-chain-id>",
		Action:    RollupCLI,
		Flags:     Flags,
	},
	{
		Name:      "deploy-config",
		Usage:     "outputs the deploy config for an L2 chain",
		Args:      true,
		ArgsUsage: "<l2-chain-id>",
		Action:    DeployConfigCLI,
		Flags:     Flags,
	},
	{
		Name:      "l2-semvers",
		Usage:     "outputs the semvers for all L2 chains",
		Args:      true,
		ArgsUsage: "<l2-chain-id>",
		Action:    L2SemversCLI,
		Flags:     Flags,
	},
	{
		Name:      "superchain-registry",
		Usage:     "outputs the .env file expected by superchain-registry add-chain tool",
		Args:      true,
		ArgsUsage: "<l2-chain-id>",
		Action:    SuperchainRegistryCLI,
		Flags:     Flags,
	},
}

type cliConfig struct {
	Workdir string
	Outfile string
	ChainID common.Hash
}

func readConfig(cliCtx *cli.Context) (cliConfig, error) {
	var cfg cliConfig

	outfile := cliCtx.String(OutfileFlagName)
	if outfile == "" {
		return cfg, fmt.Errorf("outfile flag is required")
	}

	workdir := cliCtx.String(deployer.WorkdirFlagName)
	if workdir == "" {
		return cfg, fmt.Errorf("workdir flag is required")
	}

	chainIDStr := cliCtx.Args().First()
	if chainIDStr == "" {
		return cfg, fmt.Errorf("chain-id argument is required")
	}

	chainID, err := op_service.Parse256BitChainID(chainIDStr)
	if err != nil {
		return cfg, fmt.Errorf("failed to parse chain ID: %w", err)
	}

	return cliConfig{
		Workdir: cliCtx.String(deployer.WorkdirFlagName),
		Outfile: cliCtx.String(OutfileFlagName),
		ChainID: chainID,
	}, nil
}
