package fault

import (
	"context"
	"fmt"

	"github.com/ethereum-optimism/optimism/op-challenger/config"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/claims"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/contracts"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/outputs"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/vm"
	faultTypes "github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	keccakTypes "github.com/ethereum-optimism/optimism/op-challenger/game/keccak/types"
	"github.com/ethereum-optimism/optimism/op-challenger/game/scheduler"
	"github.com/ethereum-optimism/optimism/op-challenger/metrics"
	"github.com/ethereum-optimism/optimism/op-service/clock"
	"github.com/ethereum-optimism/optimism/op-service/sources/batching"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
)

type CloseFunc func()

type Registry interface {
	RegisterGameType(gameType faultTypes.GameType, creator scheduler.PlayerCreator)
	RegisterBondContract(gameType faultTypes.GameType, creator claims.BondContractCreator)
}

type OracleRegistry interface {
	RegisterOracle(oracle keccakTypes.LargePreimageOracle)
}

type PrestateSource interface {
	// PrestatePath returns the path to the prestate file to use for the game.
	// The provided prestateHash may be used to differentiate between different states but no guarantee is made that
	// the returned prestate matches the supplied hash.
	PrestatePath(ctx context.Context, prestateHash common.Hash) (string, error)
}

type RollupClient interface {
	outputs.OutputRollupClient
	SyncStatusProvider
}

func RegisterGameTypes(
	ctx context.Context,
	systemClock clock.Clock,
	l1Clock faultTypes.ClockReader,
	logger log.Logger,
	m metrics.Metricer,
	cfg *config.Config,
	registry Registry,
	oracles OracleRegistry,
	txSender TxSender,
	gameFactory *contracts.DisputeGameFactoryContract,
	caller *batching.MultiCaller,
	l1HeaderSource L1HeaderSource,
	selective bool,
	claimants []common.Address,
) (CloseFunc, error) {
	clients := &clientProvider{ctx: ctx, logger: logger, cfg: cfg}
	var registerTasks []*RegisterTask
	if cfg.TraceTypeEnabled(faultTypes.TraceTypeCannon) {
		l2HeaderSource, rollupClient, syncValidator, err := clients.SingleChainClients()
		if err != nil {
			return nil, err
		}
		registerTasks = append(registerTasks, NewCannonRegisterTask(faultTypes.CannonGameType, cfg, m, vm.NewOpProgramServerExecutor(logger), l2HeaderSource, rollupClient, syncValidator))
	}
	if cfg.TraceTypeEnabled(faultTypes.TraceTypeSuperCannon) {
		rootProvider, syncValidator, err := clients.SuperchainClients()
		if err != nil {
			return nil, err
		}
		registerTasks = append(registerTasks, NewSuperCannonRegisterTask(faultTypes.SuperCannonGameType, cfg, m, vm.NewOpProgramServerExecutor(logger), rootProvider, syncValidator))
	}
	if cfg.TraceTypeEnabled(faultTypes.TraceTypePermissioned) {
		l2HeaderSource, rollupClient, syncValidator, err := clients.SingleChainClients()
		if err != nil {
			return nil, err
		}
		registerTasks = append(registerTasks, NewCannonRegisterTask(faultTypes.PermissionedGameType, cfg, m, vm.NewOpProgramServerExecutor(logger), l2HeaderSource, rollupClient, syncValidator))
	}
	if cfg.TraceTypeEnabled(faultTypes.TraceTypeSuperPermissioned) {
		rootProvider, syncValidator, err := clients.SuperchainClients()
		if err != nil {
			return nil, err
		}
		registerTasks = append(registerTasks, NewSuperCannonRegisterTask(faultTypes.SuperPermissionedGameType, cfg, m, vm.NewOpProgramServerExecutor(logger), rootProvider, syncValidator))
	}
	if cfg.TraceTypeEnabled(faultTypes.TraceTypeAsterisc) {
		l2HeaderSource, rollupClient, syncValidator, err := clients.SingleChainClients()
		if err != nil {
			return nil, err
		}
		registerTasks = append(registerTasks, NewAsteriscRegisterTask(faultTypes.AsteriscGameType, cfg, m, vm.NewOpProgramServerExecutor(logger), l2HeaderSource, rollupClient, syncValidator))
	}
	if cfg.TraceTypeEnabled(faultTypes.TraceTypeAsteriscKona) {
		l2HeaderSource, rollupClient, syncValidator, err := clients.SingleChainClients()
		if err != nil {
			return nil, err
		}
		registerTasks = append(registerTasks, NewAsteriscKonaRegisterTask(faultTypes.AsteriscKonaGameType, cfg, m, vm.NewKonaExecutor(), l2HeaderSource, rollupClient, syncValidator))
	}
	if cfg.TraceTypeEnabled(faultTypes.TraceTypeFast) {
		l2HeaderSource, rollupClient, syncValidator, err := clients.SingleChainClients()
		if err != nil {
			return nil, err
		}
		registerTasks = append(registerTasks, NewAlphabetRegisterTask(faultTypes.FastGameType, l2HeaderSource, rollupClient, syncValidator))
	}
	if cfg.TraceTypeEnabled(faultTypes.TraceTypeAlphabet) {
		l2HeaderSource, rollupClient, syncValidator, err := clients.SingleChainClients()
		if err != nil {
			return nil, err
		}
		registerTasks = append(registerTasks, NewAlphabetRegisterTask(faultTypes.AlphabetGameType, l2HeaderSource, rollupClient, syncValidator))
	}
	for _, task := range registerTasks {
		if err := task.Register(ctx, registry, oracles, systemClock, l1Clock, logger, m, txSender, gameFactory, caller, l1HeaderSource, selective, claimants); err != nil {
			return clients.Close, fmt.Errorf("failed to register %v game type: %w", task.gameType, err)
		}
	}
	return clients.Close, nil
}
