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
	"github.com/ethereum/go-ethereum/ethclient"
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
	rollupClient RollupClient,
	txSender TxSender,
	gameFactory *contracts.DisputeGameFactoryContract,
	caller *batching.MultiCaller,
	l1HeaderSource L1HeaderSource,
	selective bool,
	claimants []common.Address,
) (CloseFunc, error) {
	l2Client, err := ethclient.DialContext(ctx, cfg.L2Rpc)
	if err != nil {
		return nil, fmt.Errorf("dial l2 client %v: %w", cfg.L2Rpc, err)
	}
	syncValidator := newSyncStatusValidator(rollupClient)

	var registerTasks []*RegisterTask
	if cfg.TraceTypeEnabled(faultTypes.TraceTypeCannon) {
		registerTasks = append(registerTasks, NewCannonRegisterTask(faultTypes.CannonGameType, cfg, m, vm.NewOpProgramServerExecutor(logger)))
	}
	if cfg.TraceTypeEnabled(faultTypes.TraceTypePermissioned) {
		registerTasks = append(registerTasks, NewCannonRegisterTask(faultTypes.PermissionedGameType, cfg, m, vm.NewOpProgramServerExecutor(logger)))
	}
	if cfg.TraceTypeEnabled(faultTypes.TraceTypeAsterisc) {
		registerTasks = append(registerTasks, NewAsteriscRegisterTask(faultTypes.AsteriscGameType, cfg, m, vm.NewOpProgramServerExecutor(logger)))
	}
	if cfg.TraceTypeEnabled(faultTypes.TraceTypeAsteriscKona) {
		registerTasks = append(registerTasks, NewAsteriscKonaRegisterTask(faultTypes.AsteriscKonaGameType, cfg, m, vm.NewKonaExecutor()))
	}
	if cfg.TraceTypeEnabled(faultTypes.TraceTypeFast) {
		registerTasks = append(registerTasks, NewAlphabetRegisterTask(faultTypes.FastGameType))
	}
	if cfg.TraceTypeEnabled(faultTypes.TraceTypeAlphabet) {
		registerTasks = append(registerTasks, NewAlphabetRegisterTask(faultTypes.AlphabetGameType))
	}
	for _, task := range registerTasks {
		if err := task.Register(ctx, registry, oracles, systemClock, l1Clock, logger, m, syncValidator, rollupClient, txSender, gameFactory, caller, l2Client, l1HeaderSource, selective, claimants); err != nil {
			return nil, fmt.Errorf("failed to register %v game type: %w", task.gameType, err)
		}
	}
	return l2Client.Close, nil
}
