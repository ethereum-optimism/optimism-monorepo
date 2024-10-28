package fault

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"

	"github.com/ethereum-optimism/optimism/op-challenger/config"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/claims"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/contracts"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/alphabet"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/asterisc"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/cannon"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/outputs"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/prestates"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/utils"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/vm"
	faultTypes "github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	"github.com/ethereum-optimism/optimism/op-challenger/game/scheduler"
	"github.com/ethereum-optimism/optimism/op-challenger/game/types"
	"github.com/ethereum-optimism/optimism/op-challenger/metrics"
	"github.com/ethereum-optimism/optimism/op-service/clock"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/sources/batching"
	"github.com/ethereum-optimism/optimism/op-service/sources/caching"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
)

type RegisterTask struct {
	gameType               faultTypes.GameType
	skipPrestateValidation bool

	getPrestateProvider func(ctx context.Context, prestateHash common.Hash) (faultTypes.PrestateProvider, error)
	newTraceAccessor    func(
		logger log.Logger,
		m metrics.Metricer,
		l2Client utils.L2HeaderSource,
		prestateProvider faultTypes.PrestateProvider,
		vmPrestateProvider faultTypes.PrestateProvider,
		rollupClient outputs.OutputRollupClient,
		dir string,
		l1Head eth.BlockID,
		splitDepth faultTypes.Depth,
		prestateBlock uint64,
		poststateBlock uint64) (*trace.Accessor, error)
}

func NewCannonRegisterTask(gameType faultTypes.GameType, cfg *config.Config, m caching.Metrics, serverExecutor vm.OracleServerExecutor) *RegisterTask {
	stateConverter := cannon.NewStateConverter(cfg.Cannon)
	return &RegisterTask{
		gameType: gameType,
		// Don't validate the absolute prestate or genesis output root for permissioned games
		// Only trusted actors participate in these games so they aren't expected to reach the step() call and
		// are often configured without valid prestates but the challenger should still resolve the games.
		skipPrestateValidation: gameType == faultTypes.PermissionedGameType,
		getPrestateProvider: cachePrestates(
			gameType,
			stateConverter,
			m,
			cfg.CannonAbsolutePreStateBaseURL,
			cfg.CannonAbsolutePreState,
			filepath.Join(cfg.Datadir, "cannon-prestates"),
			func(ctx context.Context, path string) faultTypes.PrestateProvider {
				return vm.NewPrestateProvider(path, stateConverter)
			}),
		newTraceAccessor: func(
			logger log.Logger,
			m metrics.Metricer,
			l2Client utils.L2HeaderSource,
			prestateProvider faultTypes.PrestateProvider,
			vmPrestateProvider faultTypes.PrestateProvider,
			rollupClient outputs.OutputRollupClient,
			dir string,
			l1Head eth.BlockID,
			splitDepth faultTypes.Depth,
			prestateBlock uint64,
			poststateBlock uint64) (*trace.Accessor, error) {
			provider := vmPrestateProvider.(*vm.PrestateProvider)
			return outputs.NewOutputCannonTraceAccessor(logger, m, cfg.Cannon, serverExecutor, l2Client, prestateProvider, provider.PrestatePath(), rollupClient, dir, l1Head, splitDepth, prestateBlock, poststateBlock)
		},
	}
}

func NewAsteriscRegisterTask(gameType faultTypes.GameType, cfg *config.Config, m caching.Metrics, serverExecutor vm.OracleServerExecutor) *RegisterTask {
	stateConverter := asterisc.NewStateConverter(cfg.Asterisc)
	return &RegisterTask{
		gameType: gameType,
		getPrestateProvider: cachePrestates(
			gameType,
			stateConverter,
			m,
			cfg.AsteriscAbsolutePreStateBaseURL,
			cfg.AsteriscAbsolutePreState,
			filepath.Join(cfg.Datadir, "asterisc-prestates"),
			func(ctx context.Context, path string) faultTypes.PrestateProvider {
				return vm.NewPrestateProvider(path, stateConverter)
			}),
		newTraceAccessor: func(
			logger log.Logger,
			m metrics.Metricer,
			l2Client utils.L2HeaderSource,
			prestateProvider faultTypes.PrestateProvider,
			vmPrestateProvider faultTypes.PrestateProvider,
			rollupClient outputs.OutputRollupClient,
			dir string,
			l1Head eth.BlockID,
			splitDepth faultTypes.Depth,
			prestateBlock uint64,
			poststateBlock uint64) (*trace.Accessor, error) {
			provider := vmPrestateProvider.(*vm.PrestateProvider)
			return outputs.NewOutputAsteriscTraceAccessor(logger, m, cfg.Asterisc, serverExecutor, l2Client, prestateProvider, provider.PrestatePath(), rollupClient, dir, l1Head, splitDepth, prestateBlock, poststateBlock)
		},
	}
}

func NewAsteriscKonaRegisterTask(gameType faultTypes.GameType, cfg *config.Config, m caching.Metrics, serverExecutor vm.OracleServerExecutor) *RegisterTask {
	stateConverter := asterisc.NewStateConverter(cfg.Asterisc)
	return &RegisterTask{
		gameType: gameType,
		getPrestateProvider: cachePrestates(
			gameType,
			stateConverter,
			m,
			cfg.AsteriscKonaAbsolutePreStateBaseURL,
			cfg.AsteriscKonaAbsolutePreState,
			filepath.Join(cfg.Datadir, "asterisc-kona-prestates"),
			func(ctx context.Context, path string) faultTypes.PrestateProvider {
				return vm.NewPrestateProvider(path, stateConverter)
			}),
		newTraceAccessor: func(
			logger log.Logger,
			m metrics.Metricer,
			l2Client utils.L2HeaderSource,
			prestateProvider faultTypes.PrestateProvider,
			vmPrestateProvider faultTypes.PrestateProvider,
			rollupClient outputs.OutputRollupClient,
			dir string,
			l1Head eth.BlockID,
			splitDepth faultTypes.Depth,
			prestateBlock uint64,
			poststateBlock uint64) (*trace.Accessor, error) {
			provider := vmPrestateProvider.(*vm.PrestateProvider)
			return outputs.NewOutputAsteriscTraceAccessor(logger, m, cfg.AsteriscKona, serverExecutor, l2Client, prestateProvider, provider.PrestatePath(), rollupClient, dir, l1Head, splitDepth, prestateBlock, poststateBlock)
		},
	}
}

func NewAlphabetRegisterTask(gameType faultTypes.GameType) *RegisterTask {
	return &RegisterTask{
		gameType: gameType,
		getPrestateProvider: func(_ context.Context, _ common.Hash) (faultTypes.PrestateProvider, error) {
			return alphabet.PrestateProvider, nil
		},
		newTraceAccessor: func(
			logger log.Logger,
			m metrics.Metricer,
			l2Client utils.L2HeaderSource,
			prestateProvider faultTypes.PrestateProvider,
			vmPrestateProvider faultTypes.PrestateProvider,
			rollupClient outputs.OutputRollupClient,
			dir string,
			l1Head eth.BlockID,
			splitDepth faultTypes.Depth,
			prestateBlock uint64,
			poststateBlock uint64) (*trace.Accessor, error) {
			return outputs.NewOutputAlphabetTraceAccessor(logger, m, prestateProvider, rollupClient, l2Client, l1Head, splitDepth, prestateBlock, poststateBlock)
		},
	}
}

func cachePrestates(
	gameType faultTypes.GameType,
	stateConverter vm.StateConverter,
	m caching.Metrics,
	prestateBaseURL *url.URL,
	preStatePath string,
	prestateDir string,
	newPrestateProvider func(ctx context.Context, path string) faultTypes.PrestateProvider,
) func(ctx context.Context, prestateHash common.Hash) (faultTypes.PrestateProvider, error) {
	prestateSource := prestates.NewPrestateSource(prestateBaseURL, preStatePath, prestateDir, stateConverter)
	prestateProviderCache := prestates.NewPrestateProviderCache(m, fmt.Sprintf("prestates-%v", gameType), func(ctx context.Context, prestateHash common.Hash) (faultTypes.PrestateProvider, error) {
		prestatePath, err := prestateSource.PrestatePath(ctx, prestateHash)
		if err != nil {
			return nil, fmt.Errorf("required prestate %v not available: %w", prestateHash, err)
		}
		return newPrestateProvider(ctx, prestatePath), nil
	})
	return prestateProviderCache.GetOrCreate
}

func (e *RegisterTask) Register(
	ctx context.Context,
	registry Registry,
	oracles OracleRegistry,
	systemClock clock.Clock,
	l1Clock faultTypes.ClockReader,
	logger log.Logger,
	m metrics.Metricer,
	syncValidator SyncValidator,
	rollupClient outputs.OutputRollupClient,
	txSender TxSender,
	gameFactory *contracts.DisputeGameFactoryContract,
	caller *batching.MultiCaller,
	l2Client utils.L2HeaderSource,
	l1HeaderSource L1HeaderSource,
	selective bool,
	claimants []common.Address) error {

	playerCreator := func(game types.GameMetadata, dir string) (scheduler.GamePlayer, error) {
		contract, err := contracts.NewFaultDisputeGameContract(ctx, m, game.Proxy, caller)
		if err != nil {
			return nil, fmt.Errorf("failed to create fault dispute game contracts: %w", err)
		}
		requiredPrestatehash, err := contract.GetAbsolutePrestateHash(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to load prestate hash for game %v: %w", game.Proxy, err)
		}

		vmPrestateProvider, err := e.getPrestateProvider(ctx, requiredPrestatehash)
		if err != nil {
			return nil, fmt.Errorf("required prestate %v not available for game %v: %w", requiredPrestatehash, game.Proxy, err)
		}

		oracle, err := contract.GetOracle(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to load oracle for game %v: %w", game.Proxy, err)
		}
		oracles.RegisterOracle(oracle)
		prestateBlock, poststateBlock, err := contract.GetBlockRange(ctx)
		if err != nil {
			return nil, err
		}
		splitDepth, err := contract.GetSplitDepth(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to load split depth: %w", err)
		}
		l1HeadID, err := loadL1Head(contract, ctx, l1HeaderSource)
		if err != nil {
			return nil, err
		}
		prestateProvider := outputs.NewPrestateProvider(rollupClient, prestateBlock)
		creator := func(ctx context.Context, logger log.Logger, gameDepth faultTypes.Depth, dir string) (faultTypes.TraceAccessor, error) {
			accessor, err := e.newTraceAccessor(logger, m, l2Client, prestateProvider, vmPrestateProvider, rollupClient, dir, l1HeadID, splitDepth, prestateBlock, poststateBlock)
			if err != nil {
				return nil, err
			}
			return accessor, nil
		}
		var validators []Validator
		if !e.skipPrestateValidation {
			validators = append(validators, NewPrestateValidator(e.gameType.String(), contract.GetAbsolutePrestateHash, vmPrestateProvider))
			validators = append(validators, NewPrestateValidator("output root", contract.GetStartingRootHash, prestateProvider))
		}
		return NewGamePlayer(ctx, systemClock, l1Clock, logger, m, dir, game.Proxy, txSender, contract, syncValidator, validators, creator, l1HeaderSource, selective, claimants)
	}
	err := registerOracle(ctx, logger, m, oracles, gameFactory, caller, e.gameType)
	if err != nil {
		return err
	}
	registry.RegisterGameType(e.gameType, playerCreator)

	contractCreator := func(game types.GameMetadata) (claims.BondContract, error) {
		return contracts.NewFaultDisputeGameContract(ctx, m, game.Proxy, caller)
	}
	registry.RegisterBondContract(e.gameType, contractCreator)
	return nil
}

func registerOracle(ctx context.Context, logger log.Logger, m metrics.Metricer, oracles OracleRegistry, gameFactory *contracts.DisputeGameFactoryContract, caller *batching.MultiCaller, gameType faultTypes.GameType) error {
	implAddr, err := gameFactory.GetGameImpl(ctx, gameType)
	if err != nil {
		return fmt.Errorf("failed to load implementation for game type %v: %w", gameType, err)
	}
	if implAddr == (common.Address{}) {
		logger.Warn("No game implementation set for game type", "gameType", gameType)
		return nil
	}
	contract, err := contracts.NewFaultDisputeGameContract(ctx, m, implAddr, caller)
	if err != nil {
		return fmt.Errorf("failed to create fault dispute game contracts: %w", err)
	}
	oracle, err := contract.GetOracle(ctx)
	if err != nil {
		return fmt.Errorf("failed to load oracle address: %w", err)
	}
	oracles.RegisterOracle(oracle)
	return nil
}

func loadL1Head(contract contracts.FaultDisputeGameContract, ctx context.Context, l1HeaderSource L1HeaderSource) (eth.BlockID, error) {
	l1Head, err := contract.GetL1Head(ctx)
	if err != nil {
		return eth.BlockID{}, fmt.Errorf("failed to load L1 head: %w", err)
	}
	l1Header, err := l1HeaderSource.HeaderByHash(ctx, l1Head)
	if err != nil {
		return eth.BlockID{}, fmt.Errorf("failed to load L1 header: %w", err)
	}
	return eth.HeaderBlockID(l1Header), nil
}
