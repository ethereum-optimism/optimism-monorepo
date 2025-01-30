package dripper

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v2"

	"github.com/ethereum-optimism/optimism/op-dripper/flags"
	opservice "github.com/ethereum-optimism/optimism/op-service"
	"github.com/ethereum-optimism/optimism/op-service/cliapp"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
)

func Main(version string) cliapp.LifecycleAction {
	return func(cliCtx *cli.Context, _ context.CancelCauseFunc) (cliapp.Lifecycle, error) {
		if err := flags.CheckRequired(cliCtx); err != nil {
			return nil, err
		}
		cfg := NewConfig(cliCtx)
		if err := cfg.Check(); err != nil {
			return nil, fmt.Errorf("invalid CLI flags: %w", err)
		}

		l := oplog.NewLogger(oplog.AppOut(cliCtx), cfg.LogConfig)
		oplog.SetGlobalLogHandler(l.Handler())
		opservice.ValidateEnvVars(flags.EnvVarPrefix, flags.Flags, l)

		l.Info("initializing executor")
		return DripExecutorServiceFromCLIConfig(cliCtx.Context, version, cfg, l)
	}
}
