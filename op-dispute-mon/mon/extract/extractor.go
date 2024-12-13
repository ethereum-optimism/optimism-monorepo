package extract

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	gameTypes "github.com/ethereum-optimism/optimism/op-challenger/game/types"
	monTypes "github.com/ethereum-optimism/optimism/op-dispute-mon/mon/types"
	"github.com/ethereum-optimism/optimism/op-service/clock"
	"github.com/ethereum-optimism/optimism/op-service/sources/batching/rpcblock"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"golang.org/x/exp/maps"
)

var (
	ErrIgnored = errors.New("ignored")
)

type (
	CreateGameCaller   func(ctx context.Context, game gameTypes.GameMetadata) (GameCaller, error)
	FactoryGameFetcher func(ctx context.Context, blockHash common.Hash, earliestTimestamp uint64) ([]gameTypes.GameMetadata, error)
)

type Enricher interface {
	Enrich(ctx context.Context, block rpcblock.Block, caller GameCaller, game *monTypes.EnrichedGameData) error
}

type Extractor struct {
	logger         log.Logger
	clock          clock.Clock
	createContract CreateGameCaller
	fetchGames     FactoryGameFetcher
	maxConcurrency int
	enrichers      []Enricher
	ignoredGames   map[common.Address]bool
	latestGameData map[common.Address]*monTypes.EnrichedGameData
}

func NewExtractor(logger log.Logger, cl clock.Clock, creator CreateGameCaller, fetchGames FactoryGameFetcher, ignoredGames []common.Address, maxConcurrency uint, enrichers ...Enricher) *Extractor {
	ignored := make(map[common.Address]bool)
	for _, game := range ignoredGames {
		ignored[game] = true
	}
	return &Extractor{
		logger:         logger,
		clock:          cl,
		createContract: creator,
		fetchGames:     fetchGames,
		maxConcurrency: int(maxConcurrency),
		enrichers:      enrichers,
		ignoredGames:   ignored,
	}
}

func (e *Extractor) Extract(ctx context.Context, blockHash common.Hash, minTimestamp uint64) ([]*monTypes.EnrichedGameData, int, int, error) {
	games, err := e.fetchGames(ctx, blockHash, minTimestamp)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to load games: %w", err)
	}
	enriched, ignored, failed := e.enrichGames(ctx, blockHash, games)
	return enriched, ignored, failed, nil
}

func (e *Extractor) enrichGames(ctx context.Context, blockHash common.Hash, games []gameTypes.GameMetadata) ([]*monTypes.EnrichedGameData, int, int) {
	var ignored atomic.Int32
	var failed atomic.Int32

	var wg sync.WaitGroup
	wg.Add(e.maxConcurrency)
	gameCh := make(chan gameTypes.GameMetadata, e.maxConcurrency)
	// Create a channel for enriched games. Must have enough capacity to hold all games.
	enrichedCh := make(chan *monTypes.EnrichedGameData, len(games))
	// Spin up multiple goroutines to enrich game data
	for i := 0; i < e.maxConcurrency; i++ {
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case game, ok := <-gameCh:
					if !ok {
						e.logger.Debug("Enriching complete")
						// Channel closed
						return
					}
					e.logger.Trace("Enriching game", "game", game.Proxy)
					enrichedGame, err := e.enrichGame(ctx, blockHash, game)
					if errors.Is(err, ErrIgnored) {
						ignored.Add(1)
						e.logger.Warn("Ignoring game", "game", game.Proxy)
						continue
					} else if err != nil {
						failed.Add(1)
						e.logger.Error("Failed to fetch game data", "game", game.Proxy, "err", err)
						continue
					}
					enrichedCh <- enrichedGame
				}
			}
		}()
	}

	// Create a new store for game data. This ensures any games no longer in the monitoring set are dropped.
	updatedGameData := make(map[common.Address]*monTypes.EnrichedGameData)
	// Push each game into the channel and store the latest cached game data as a default if fetching fails
	for _, game := range games {
		previousData := e.latestGameData[game.Proxy]
		if previousData != nil {
			updatedGameData[game.Proxy] = previousData
		}
		gameCh <- game
	}
	close(gameCh)
	// Wait for games to finish being enriched then close enrichedCh since no future results will be published
	wg.Wait()
	close(enrichedCh)

	// Read the results
	for enrichedGame := range enrichedCh {
		updatedGameData[enrichedGame.Proxy] = enrichedGame
	}
	e.latestGameData = updatedGameData
	return maps.Values(updatedGameData), int(ignored.Load()), int(failed.Load())
}

func (e *Extractor) enrichGame(ctx context.Context, blockHash common.Hash, game gameTypes.GameMetadata) (*monTypes.EnrichedGameData, error) {
	if e.ignoredGames[game.Proxy] {
		return nil, ErrIgnored
	}
	caller, err := e.createContract(ctx, game)
	if err != nil {
		return nil, fmt.Errorf("failed to create contracts: %w", err)
	}
	meta, err := caller.GetGameMetadata(ctx, rpcblock.ByHash(blockHash))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch game metadata: %w", err)
	}
	claims, err := caller.GetAllClaims(ctx, rpcblock.ByHash(blockHash))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch game claims: %w", err)
	}
	enrichedClaims := make([]monTypes.EnrichedClaim, len(claims))
	for i, claim := range claims {
		enrichedClaims[i] = monTypes.EnrichedClaim{Claim: claim}
	}
	enrichedGame := &monTypes.EnrichedGameData{
		LastUpdateTime:        e.clock.Now(),
		GameMetadata:          game,
		L1Head:                meta.L1Head,
		L2BlockNumber:         meta.L2BlockNum,
		RootClaim:             meta.RootClaim,
		Status:                meta.Status,
		MaxClockDuration:      meta.MaxClockDuration,
		BlockNumberChallenged: meta.L2BlockNumberChallenged,
		BlockNumberChallenger: meta.L2BlockNumberChallenger,
		Claims:                enrichedClaims,
	}
	if err := e.applyEnrichers(ctx, blockHash, caller, enrichedGame); err != nil {
		return nil, fmt.Errorf("failed to enrich game: %w", err)
	}
	return enrichedGame, nil
}

func (e *Extractor) applyEnrichers(ctx context.Context, blockHash common.Hash, caller GameCaller, game *monTypes.EnrichedGameData) error {
	for _, enricher := range e.enrichers {
		if err := enricher.Enrich(ctx, rpcblock.ByHash(blockHash), caller, game); err != nil {
			return err
		}
	}
	return nil
}
