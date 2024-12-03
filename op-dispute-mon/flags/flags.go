package flags

import (
	"fmt"

	challengerFlags "github.com/ethereum-optimism/optimism/op-challenger/flags"
	"github.com/ethereum-optimism/optimism/op-service/flags"
	"github.com/urfave/cli/v2"

	"github.com/ethereum-optimism/optimism/op-dispute-mon/config"
	opservice "github.com/ethereum-optimism/optimism/op-service"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	opmetrics "github.com/ethereum-optimism/optimism/op-service/metrics"
	"github.com/ethereum-optimism/optimism/op-service/oppprof"
	"github.com/ethereum/go-ethereum/common"
)

const (
	envVarPrefix = "OP_DISPUTE_MON"
)

func prefixEnvVars(name string) []string {
	return opservice.PrefixEnvVar(envVarPrefix, name)
}

var (
	// Required Flags
	L1EthRpcFlag = &cli.StringFlag{
		Name:     "l1-eth-rpc",
		Usage:    "HTTP provider URL for L1.",
		EnvVars:  prefixEnvVars("L1_ETH_RPC"),
		Required: true,
	}
	RollupRpcFlag = &cli.StringFlag{
		Name:     "rollup-rpc",
		Usage:    "HTTP provider URL for the rollup node",
		EnvVars:  prefixEnvVars("ROLLUP_RPC"),
		Required: true,
	}
	// Optional Flags
	GameFactoryAddressFlag = &cli.StringFlag{
		Name:    "game-factory-address",
		Usage:   "Address of the fault game factory contract.",
		EnvVars: prefixEnvVars("GAME_FACTORY_ADDRESS"),
	}
	NetworkFlag      = flags.CLINetworkFlag(envVarPrefix, "")
	HonestActorsFlag = &cli.StringSliceFlag{
		Name:    "honest-actors",
		Usage:   "List of honest actors that are monitored for any claims that are resolved against them.",
		EnvVars: prefixEnvVars("HONEST_ACTORS"),
	}
	MonitorIntervalFlag = &cli.DurationFlag{
		Name:    "monitor-interval",
		Usage:   "The interval at which the dispute monitor will check for new games to monitor.",
		EnvVars: prefixEnvVars("MONITOR_INTERVAL"),
		Value:   config.DefaultMonitorInterval,
	}
	GameWindowFlag = &cli.DurationFlag{
		Name: "game-window",
		Usage: "The time window which the monitor will consider games to report on. " +
			"This should include a bond claim buffer for games outside the maximum game duration.",
		EnvVars: prefixEnvVars("GAME_WINDOW"),
		Value:   config.DefaultGameWindow,
	}
	IgnoredGamesFlag = &cli.StringSliceFlag{
		Name:    "ignored-games",
		Usage:   "List of game addresses to exclude from monitoring.",
		EnvVars: prefixEnvVars("IGNORED_GAMES"),
	}
	MaxConcurrencyFlag = &cli.UintFlag{
		Name:    "max-concurrency",
		Usage:   "Maximum number of threads to use when fetching game data",
		EnvVars: prefixEnvVars("MAX_CONCURRENCY"),
		Value:   config.DefaultMaxConcurrency,
	}
)

// Flags contains the list of configuration options available to the binary.
var Flags = []cli.Flag{
	L1EthRpcFlag,
	RollupRpcFlag,
	GameFactoryAddressFlag,
	NetworkFlag,
	HonestActorsFlag,
	MonitorIntervalFlag,
	GameWindowFlag,
	IgnoredGamesFlag,
	MaxConcurrencyFlag,
}

func init() {
	Flags = append(Flags, oplog.CLIFlags(envVarPrefix)...)
	Flags = append(Flags, opmetrics.CLIFlags(envVarPrefix)...)
	Flags = append(Flags, oppprof.CLIFlags(envVarPrefix)...)
}

// NewConfigFromCLI parses the Config from the provided flags or environment variables.
func NewConfigFromCLI(ctx *cli.Context) (*config.Config, error) {
	gameFactoryAddress, err := challengerFlags.FactoryAddress(ctx)
	if err != nil {
		return nil, err
	}

	var actors []common.Address
	if ctx.IsSet(HonestActorsFlag.Name) {
		for _, addrStr := range ctx.StringSlice(HonestActorsFlag.Name) {
			actor, err := opservice.ParseAddress(addrStr)
			if err != nil {
				return nil, fmt.Errorf("invalid honest actor address: %w", err)
			}
			actors = append(actors, actor)
		}
	}

	var ignoredGames []common.Address
	if ctx.IsSet(IgnoredGamesFlag.Name) {
		for _, addrStr := range ctx.StringSlice(IgnoredGamesFlag.Name) {
			game, err := opservice.ParseAddress(addrStr)
			if err != nil {
				return nil, fmt.Errorf("invalid ignored game address: %w", err)
			}
			ignoredGames = append(ignoredGames, game)
		}
	}

	maxConcurrency := ctx.Uint(MaxConcurrencyFlag.Name)
	if maxConcurrency == 0 {
		return nil, fmt.Errorf("%v must not be 0", MaxConcurrencyFlag.Name)
	}

	metricsConfig := opmetrics.ReadCLIConfig(ctx)
	pprofConfig := oppprof.ReadCLIConfig(ctx)

	return &config.Config{
		L1EthRpc:           ctx.String(L1EthRpcFlag.Name),
		GameFactoryAddress: gameFactoryAddress,
		RollupRpc:          ctx.String(RollupRpcFlag.Name),

		HonestActors:    actors,
		MonitorInterval: ctx.Duration(MonitorIntervalFlag.Name),
		GameWindow:      ctx.Duration(GameWindowFlag.Name),
		IgnoredGames:    ignoredGames,
		MaxConcurrency:  maxConcurrency,

		MetricsConfig: metricsConfig,
		PprofConfig:   pprofConfig,
	}, nil
}
