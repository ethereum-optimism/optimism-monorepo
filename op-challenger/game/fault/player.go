package fault

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/claims"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/contracts"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/preimages"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/responder"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	gameTypes "github.com/ethereum-optimism/optimism/op-challenger/game/types"
	"github.com/ethereum-optimism/optimism/op-challenger/metrics"
	"github.com/ethereum-optimism/optimism/op-service/clock"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/txmgr"
	"github.com/ethereum/go-ethereum/common"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
)

type actor func(ctx context.Context) error

type GameInfo interface {
	GetStatus(context.Context) (gameTypes.GameStatus, error)
	GetClaimCount(context.Context) (uint64, error)
}

type SyncValidator interface {
	// ValidateNodeSynced checks that the local node is sufficiently up to date to play the game.
	// It returns types.ErrNotInSync if the node is too far behind.
	ValidateNodeSynced(ctx context.Context, gameL1Head eth.BlockID) error
}

type L1HeaderSource interface {
	HeaderByHash(context.Context, common.Hash) (*gethTypes.Header, error)
}

type TxSender interface {
	From() common.Address
	SendAndWaitSimple(txPurpose string, txs ...txmgr.TxCandidate) error
}

type GamePlayer struct {
	act                actor
	loader             GameInfo
	logger             log.Logger
	syncValidator      SyncValidator
	prestateValidators []Validator
	status             gameTypes.GameStatus
	gameL1Head         eth.BlockID
}

type GameContract interface {
	preimages.PreimageGameContract
	responder.GameContract
	claims.BondContract
	GameInfo
	ClaimLoader
	GetStatus(ctx context.Context) (gameTypes.GameStatus, error)
	GetMaxGameDepth(ctx context.Context) (types.Depth, error)
	GetMaxClockDuration(ctx context.Context) (time.Duration, error)
	GetOracle(ctx context.Context) (contracts.PreimageOracleContract, error)
	GetL1Head(ctx context.Context) (common.Hash, error)
}

var actNoop = func(ctx context.Context) error {
	return nil
}

type resourceCreator func(ctx context.Context, logger log.Logger, gameDepth types.Depth, dir string) (types.TraceAccessor, error)

func NewGamePlayer(
	ctx context.Context,
	systemClock clock.Clock,
	l1Clock types.ClockReader,
	logger log.Logger,
	m metrics.Metricer,
	dir string,
	addr common.Address,
	txSender TxSender,
	loader GameContract,
	syncValidator SyncValidator,
	validators []Validator,
	creator resourceCreator,
	l1HeaderSource L1HeaderSource,
	selective bool,
	claimants []common.Address,
) (*GamePlayer, error) {
	logger = logger.New("game", addr)

	status, err := loader.GetStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch game status: %w", err)
	}
	if status != gameTypes.GameStatusInProgress {
		logger.Info("Game already resolved", "status", status)
		// Game is already complete so skip creating the trace provider, loading game inputs etc.
		return &GamePlayer{
			logger:             logger,
			loader:             loader,
			prestateValidators: validators,
			status:             status,
			// Act function does nothing because the game is already complete
			act: actNoop,
		}, nil
	}

	maxClockDuration, err := loader.GetMaxClockDuration(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch the game duration: %w", err)
	}

	gameDepth, err := loader.GetMaxGameDepth(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch the game depth: %w", err)
	}

	accessor, err := creator(ctx, logger, gameDepth, dir)
	if err != nil {
		return nil, fmt.Errorf("failed to create trace accessor: %w", err)
	}

	oracle, err := loader.GetOracle(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load oracle: %w", err)
	}

	l1HeadHash, err := loader.GetL1Head(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load game L1 head: %w", err)
	}
	l1Header, err := l1HeaderSource.HeaderByHash(ctx, l1HeadHash)
	if err != nil {
		return nil, fmt.Errorf("failed to load L1 header %v: %w", l1HeadHash, err)
	}
	l1Head := eth.HeaderBlockID(l1Header)

	minLargePreimageSize, err := oracle.MinLargePreimageSize(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load min large preimage size: %w", err)
	}
	direct := preimages.NewDirectPreimageUploader(logger, txSender, loader)
	large := preimages.NewLargePreimageUploader(logger, l1Clock, txSender, oracle)
	uploader := preimages.NewSplitPreimageUploader(direct, large, minLargePreimageSize)
	responder, err := responder.NewFaultResponder(logger, txSender, loader, uploader, oracle)
	if err != nil {
		return nil, fmt.Errorf("failed to create the responder: %w", err)
	}

	agent := NewAgent(m, systemClock, l1Clock, loader, gameDepth, maxClockDuration, accessor, responder, logger, selective, claimants)
	return &GamePlayer{
		act:                agent.Act,
		loader:             loader,
		logger:             logger,
		status:             status,
		gameL1Head:         l1Head,
		syncValidator:      syncValidator,
		prestateValidators: validators,
	}, nil
}

func (g *GamePlayer) ValidatePrestate(ctx context.Context) error {
	for _, validator := range g.prestateValidators {
		if err := validator.Validate(ctx); err != nil {
			return fmt.Errorf("failed to validate prestate: %w", err)
		}
	}
	return nil
}

func (g *GamePlayer) Status() gameTypes.GameStatus {
	return g.status
}

func (g *GamePlayer) ProgressGame(ctx context.Context) gameTypes.GameStatus {
	if g.status != gameTypes.GameStatusInProgress {
		// Game is already complete so don't try to perform further actions.
		g.logger.Trace("Skipping completed game")
		return g.status
	}
	if err := g.syncValidator.ValidateNodeSynced(ctx, g.gameL1Head); errors.Is(err, types.ErrNotInSync) {
		g.logger.Warn("Local node not sufficiently up to date", "err", err)
		return g.status
	} else if err != nil {
		g.logger.Error("Could not check local node was in sync", "err", err)
		return g.status
	}
	g.logger.Trace("Checking if actions are required")
	if err := g.act(ctx); err != nil {
		g.logger.Error("Error when acting on game", "err", err)
	}
	status, err := g.loader.GetStatus(ctx)
	if err != nil {
		g.logger.Error("Unable to retrieve game status", "err", err)
		return gameTypes.GameStatusInProgress
	}
	g.logGameStatus(ctx, status)
	g.status = status
	if status != gameTypes.GameStatusInProgress {
		// Release the agent as we will no longer need to act on this game.
		g.act = actNoop
	}
	return status
}

func (g *GamePlayer) logGameStatus(ctx context.Context, status gameTypes.GameStatus) {
	if status == gameTypes.GameStatusInProgress {
		claimCount, err := g.loader.GetClaimCount(ctx)
		if err != nil {
			g.logger.Error("Failed to get claim count for in progress game", "err", err)
			return
		}
		g.logger.Info("Game info", "claims", claimCount, "status", status)
		return
	}
	g.logger.Info("Game resolved", "status", status)
}
