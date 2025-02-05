package main

import (
	"context"
	"fmt"

	"github.com/ethereum-optimism/optimism/op-challenger/flags"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/contracts"
	contractMetrics "github.com/ethereum-optimism/optimism/op-challenger/game/fault/contracts/metrics"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	"github.com/ethereum-optimism/optimism/op-challenger/tools"
	opservice "github.com/ethereum-optimism/optimism/op-service"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	"github.com/ethereum-optimism/optimism/op-service/sources/batching"
	"github.com/ethereum-optimism/optimism/op-service/txmgr"
	"github.com/ethereum/go-ethereum/common"
	"github.com/urfave/cli/v2"
)

var (
	GameTypeFlag = &cli.StringFlag{
		Name:    "game-type",
		Usage:   "Game type to create (numeric values).",
		EnvVars: opservice.PrefixEnvVar(flags.EnvVarPrefix, "TRACE_TYPE"),
		Value:   types.CannonGameType.String(),
	}
	OutputRootFlag = &cli.StringFlag{
		Name:    "output-root",
		Usage:   "The output root for the fault dispute game.",
		EnvVars: opservice.PrefixEnvVar(flags.EnvVarPrefix, "OUTPUT_ROOT"),
	}
	L2BlockNumFlag = &cli.StringFlag{
		Name:    "l2-block-num",
		Usage:   "The l2 block number for the game.",
		EnvVars: opservice.PrefixEnvVar(flags.EnvVarPrefix, "L2_BLOCK_NUM"),
	}
)

func CreateGame(ctx *cli.Context) error {
	outputRoot := common.HexToHash(ctx.String(OutputRootFlag.Name))
	gameType := ctx.Uint64(GameTypeFlag.Name)
	l2BlockNum := ctx.Uint64(L2BlockNumFlag.Name)

	contract, txMgr, err := NewContractWithTxMgr[*contracts.DisputeGameFactoryContract](ctx, flags.FactoryAddress,
		func(ctx context.Context, metricer contractMetrics.ContractMetricer, address common.Address, caller *batching.MultiCaller) (*contracts.DisputeGameFactoryContract, error) {
			return contracts.NewDisputeGameFactoryContract(metricer, address, caller), nil
		})
	if err != nil {
		return fmt.Errorf("failed to create dispute game factory bindings: %w", err)
	}

	creator := tools.NewGameCreator(contract, txMgr)
	gameAddr, err := creator.CreateGame(ctx.Context, outputRoot, gameType, l2BlockNum)
	if err != nil {
		return fmt.Errorf("failed to create game: %w", err)
	}
	fmt.Printf("Fetched Game Address: %s\n", gameAddr.String())
	return nil
}

func createGameFlags() []cli.Flag {
	cliFlags := []cli.Flag{
		flags.L1EthRpcFlag,
		flags.NetworkFlag,
		flags.FactoryAddressFlag,
		GameTypeFlag,
		OutputRootFlag,
		L2BlockNumFlag,
	}
	cliFlags = append(cliFlags, txmgr.CLIFlagsWithDefaults(flags.EnvVarPrefix, txmgr.DefaultChallengerFlagValues)...)
	cliFlags = append(cliFlags, oplog.CLIFlags(flags.EnvVarPrefix)...)
	return cliFlags
}

var CreateGameCommand = &cli.Command{
	Name:        "create-game",
	Usage:       "Creates a dispute game via the factory",
	Description: "Creates a dispute game via the factory",
	Action:      Interruptible(CreateGame),
	Flags:       createGameFlags(),
}
