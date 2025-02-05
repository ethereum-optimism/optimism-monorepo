package game

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum-optimism/optimism/op-challenger/game/scheduler"
	"github.com/ethereum-optimism/optimism/op-challenger/game/types"
	"github.com/ethereum-optimism/optimism/op-service/clock"
	"github.com/ethereum-optimism/optimism/op-service/eth"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
)

// gameSource loads information about the games available to play
type gameSource interface {
	GetGamesAtOrAfter(ctx context.Context, blockHash common.Hash, earliestTimestamp uint64) ([]types.GameMetadata, error)
}

type RWClock interface {
	SetTime(uint64)
	Now() time.Time
}

type gameScheduler interface {
	Schedule([]types.GameMetadata, uint64) error
}

type preimageScheduler interface {
	Schedule(blockHash common.Hash, blockNumber uint64) error
}

type claimer interface {
	Schedule(blockNumber uint64, games []types.GameMetadata) error
}

type gameMonitor struct {
	logger       log.Logger
	clock        RWClock
	source       gameSource
	scheduler    gameScheduler
	preimages    preimageScheduler
	gameWindow   time.Duration
	claimer      claimer
	allowedGames []common.Address
	l1HeadsSub   ethereum.Subscription
	l1Source     *headSource
	runState     sync.Mutex
}

type MinimalSubscriber interface {
	Subscribe(ctx context.Context, namespace string, channel interface{}, args ...interface{}) (ethereum.Subscription, error)
}

type headSource struct {
	inner MinimalSubscriber
}

func (s *headSource) SubscribeNewHead(ctx context.Context, ch chan<- *ethTypes.Header) (ethereum.Subscription, error) {
	return s.inner.Subscribe(ctx, "eth", ch, "newHeads")
}

func newGameMonitor(
	logger log.Logger,
	cl RWClock,
	source gameSource,
	scheduler gameScheduler,
	preimages preimageScheduler,
	gameWindow time.Duration,
	claimer claimer,
	allowedGames []common.Address,
	l1Source MinimalSubscriber,
) *gameMonitor {
	return &gameMonitor{
		logger:       logger,
		clock:        cl,
		scheduler:    scheduler,
		preimages:    preimages,
		source:       source,
		gameWindow:   gameWindow,
		claimer:      claimer,
		allowedGames: allowedGames,
		l1Source:     &headSource{inner: l1Source},
	}
}

func (m *gameMonitor) allowedGame(game common.Address) bool {
	if len(m.allowedGames) == 0 {
		return true
	}
	for _, allowed := range m.allowedGames {
		if allowed == game {
			return true
		}
	}
	return false
}

func (m *gameMonitor) progressGames(ctx context.Context, blockHash common.Hash, blockNumber uint64) error {
	minGameTimestamp := clock.MinCheckedTimestamp(m.clock, m.gameWindow)
	games, err := m.source.GetGamesAtOrAfter(ctx, blockHash, minGameTimestamp)
	if err != nil {
		return fmt.Errorf("failed to load games: %w", err)
	}
	var gamesToPlay []types.GameMetadata
	for _, game := range games {
		if !m.allowedGame(game.Proxy) {
			m.logger.Debug("Skipping game not on allow list", "game", game.Proxy)
			continue
		}
		gamesToPlay = append(gamesToPlay, game)
	}
	if err := m.claimer.Schedule(blockNumber, gamesToPlay); err != nil {
		return fmt.Errorf("failed to schedule bond claims: %w", err)
	}
	if err := m.scheduler.Schedule(gamesToPlay, blockNumber); errors.Is(err, scheduler.ErrBusy) {
		m.logger.Info("Scheduler still busy with previous update")
	} else if err != nil {
		return fmt.Errorf("failed to schedule games: %w", err)
	}
	return nil
}

func (m *gameMonitor) onNewL1Head(ctx context.Context, sig eth.L1BlockRef) {
	m.clock.SetTime(sig.Time)
	if err := m.progressGames(ctx, sig.Hash, sig.Number); err != nil {
		m.logger.Error("Failed to progress games", "err", err)
	}
	if err := m.preimages.Schedule(sig.Hash, sig.Number); err != nil {
		m.logger.Error("Failed to validate large preimages", "err", err)
	}
}

func (m *gameMonitor) resubscribeFunction() event.ResubscribeErrFunc {
	// The ctx is cancelled as soon as the subscription is returned,
	// but is only used to create the subscription, and does not affect the returned subscription.
	return func(ctx context.Context, err error) (event.Subscription, error) {
		if err != nil {
			m.logger.Warn("resubscribing after failed L1 subscription", "err", err)
		}
		return eth.WatchHeadChanges(ctx, m.l1Source, m.onNewL1Head)
	}
}

func (m *gameMonitor) StartMonitoring() {
	m.runState.Lock()
	defer m.runState.Unlock()
	if m.l1HeadsSub != nil {
		return // already started
	}
	m.l1HeadsSub = event.ResubscribeErr(time.Second*10, m.resubscribeFunction())
}

func (m *gameMonitor) StopMonitoring() {
	m.runState.Lock()
	defer m.runState.Unlock()
	if m.l1HeadsSub == nil {
		return // already stopped
	}
	m.l1HeadsSub.Unsubscribe()
	m.l1HeadsSub = nil
}
