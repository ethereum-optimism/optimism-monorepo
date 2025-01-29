package runner

import (
	"context"
	"errors"
	"net/url"

	"github.com/ethereum-optimism/optimism/op-challenger/config"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/asterisc"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/cannon"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/utils"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/vm"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	"github.com/ethereum/go-ethereum/log"
)

type prestateFetcher interface {
	getPrestate(ctx context.Context, logger log.Logger, prestateBaseUrl *url.URL, prestatePath string, dataDir string, stateConverter vm.StateConverter) (string, error)
}

func createTraceProvider(
	ctx context.Context,
	logger log.Logger,
	m vm.Metricer,
	cfg *config.Config,
	prestateSource prestateFetcher,
	traceType types.TraceType,
	localInputs utils.LocalGameInputs,
	dir string,
) (types.TraceProvider, error) {
	switch traceType {
	case types.TraceTypeCannon:
		serverExecutor := vm.NewOpProgramServerExecutor(logger)
		stateConverter := cannon.NewStateConverter(cfg.Cannon)
		prestate, err := prestateSource.getPrestate(ctx, logger, cfg.CannonAbsolutePreStateBaseURL, cfg.CannonAbsolutePreState, dir, stateConverter)
		if err != nil {
			return nil, err
		}
		prestateProvider := vm.NewPrestateProvider(prestate, stateConverter)
		return cannon.NewTraceProvider(logger, m, cfg.Cannon, serverExecutor, prestateProvider, prestate, localInputs, dir, 42), nil
	case types.TraceTypeAsterisc:
		serverExecutor := vm.NewOpProgramServerExecutor(logger)
		stateConverter := asterisc.NewStateConverter(cfg.Asterisc)
		prestate, err := prestateSource.getPrestate(ctx, logger, cfg.AsteriscAbsolutePreStateBaseURL, cfg.AsteriscAbsolutePreState, dir, stateConverter)
		if err != nil {
			return nil, err
		}
		prestateProvider := vm.NewPrestateProvider(prestate, stateConverter)
		return asterisc.NewTraceProvider(logger, m, cfg.Asterisc, serverExecutor, prestateProvider, prestate, localInputs, dir, 42), nil
	case types.TraceTypeAsteriscKona:
		serverExecutor := vm.NewKonaExecutor()
		stateConverter := asterisc.NewStateConverter(cfg.Asterisc)
		prestate, err := prestateSource.getPrestate(ctx, logger, cfg.AsteriscKonaAbsolutePreStateBaseURL, cfg.AsteriscKonaAbsolutePreState, dir, stateConverter)
		if err != nil {
			return nil, err
		}
		prestateProvider := vm.NewPrestateProvider(prestate, stateConverter)
		return asterisc.NewTraceProvider(logger, m, cfg.AsteriscKona, serverExecutor, prestateProvider, prestate, localInputs, dir, 42), nil
	}
	return nil, errors.New("invalid trace type")
}
