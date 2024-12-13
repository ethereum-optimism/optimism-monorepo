package main

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/ethereum-optimism/optimism/op-challenger/flags"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	"github.com/ethereum-optimism/optimism/op-challenger/runner"
	opservice "github.com/ethereum-optimism/optimism/op-service"
	"github.com/ethereum-optimism/optimism/op-service/cliapp"
	"github.com/ethereum/go-ethereum/common"
	"github.com/urfave/cli/v2"
)

var (
	ErrUnknownTraceType    = errors.New("unknown trace type")
	ErrInvalidPrestateHash = errors.New("invalid prestate hash")
)

func RunTrace(ctx *cli.Context, _ context.CancelCauseFunc) (cliapp.Lifecycle, error) {
	logger, err := setupLogging(ctx)
	if err != nil {
		return nil, err
	}
	logger.Info("Starting trace runner", "version", VersionWithMeta)

	cfg, err := flags.NewConfigFromCLI(ctx, logger)
	if err != nil {
		return nil, err
	}
	if err := cfg.Check(); err != nil {
		return nil, err
	}
	runConfigs, err := parseRunArgs(ctx.StringSlice(RunTraceRunFlag.Name))
	if err != nil {
		return nil, err
	}
	if len(runConfigs) == 0 {
		// Default to running on-chain version of each enabled trace type
		for _, traceType := range cfg.TraceTypes {
			runConfigs = append(runConfigs, runner.RunConfig{TraceType: traceType})
		}
	}
	return runner.NewRunner(logger, cfg, runConfigs), nil
}

func runTraceFlags() []cli.Flag {
	return append(flags.Flags, RunTraceRunFlag)
}

var RunTraceCommand = &cli.Command{
	Name:        "run-trace",
	Usage:       "Continuously runs the specified trace providers in a regular loop",
	Description: "Runs trace providers against real chain data to confirm compatibility",
	Action:      cliapp.LifecycleCmd(RunTrace),
	Flags:       runTraceFlags(),
}

var (
	RunTraceRunFlag = &cli.StringSliceFlag{
		Name: "run",
		Usage: "Specify a trace to run. Format is traceType/name/prestateHash where " +
			"traceType is the trace type to use with the prestate (e.g cannon or asterisc-kona), " +
			"name is an arbitrary name for the prestate to use when reporting metrics and" +
			"prestateHash is the hex encoded absolute prestate commitment to use. " +
			"If name is omitted the trace type name is used." +
			"If the prestateHash is omitted, the absolute prestate hash used for new games on-chain.",
		EnvVars: opservice.PrefixEnvVar(flags.EnvVarPrefix, "RUN"),
	}
)

func parseRunArgs(args []string) ([]runner.RunConfig, error) {
	cfgs := make([]runner.RunConfig, len(args))
	for i, arg := range args {
		cfg, err := parseRunArg(arg)
		if err != nil {
			return nil, err
		}
		cfgs[i] = cfg
	}
	return cfgs, nil
}

func parseRunArg(arg string) (runner.RunConfig, error) {
	cfg := runner.RunConfig{}
	opts := strings.SplitN(arg, "/", 3)
	if len(opts) == 0 {
		return runner.RunConfig{}, fmt.Errorf("invalid run config %q", arg)
	}
	cfg.TraceType = types.TraceType(opts[0])
	if !slices.Contains(types.TraceTypes, cfg.TraceType) {
		return runner.RunConfig{}, fmt.Errorf("%w %q for run config %q", ErrUnknownTraceType, opts[0], arg)
	}
	if len(opts) > 1 {
		cfg.Name = opts[1]
	} else {
		cfg.Name = cfg.TraceType.String()
	}
	if len(opts) > 2 {
		cfg.Prestate = common.HexToHash(opts[2])
		if cfg.Prestate == (common.Hash{}) {
			return runner.RunConfig{}, fmt.Errorf("%w %q for run config %q", ErrInvalidPrestateHash, opts[2], arg)
		}
	}
	return cfg, nil
}
