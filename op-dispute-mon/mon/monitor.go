package mon

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum-optimism/optimism/op-dispute-mon/mon/types"
	"github.com/ethereum-optimism/optimism/op-service/clock"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
)

type ForecastResolution func(games []*types.EnrichedGameData, ignoredCount, failedCount int)
type Monitor func(games []*types.EnrichedGameData)
type BlockHashFetcher func(ctx context.Context, number *big.Int) (common.Hash, error)
type BlockNumberFetcher func(ctx context.Context) (uint64, error)
type Extract func(ctx context.Context, blockHash common.Hash, minTimestamp uint64) ([]*types.EnrichedGameData, int, int, error)

type MonitorMetrics interface {
	RecordMonitorDuration(dur time.Duration)
}

type gameMonitor struct {
	logger  log.Logger
	clock   clock.Clock
	metrics MonitorMetrics

	done   chan struct{}
	ctx    context.Context
	cancel context.CancelFunc

	gameWindow      time.Duration
	monitorInterval time.Duration

	forecast         ForecastResolution
	monitors         []Monitor
	extract          Extract
	fetchBlockHash   BlockHashFetcher
	fetchBlockNumber BlockNumberFetcher
}

func newGameMonitor(
	ctx context.Context,
	logger log.Logger,
	cl clock.Clock,
	metrics MonitorMetrics,
	monitorInterval time.Duration,
	gameWindow time.Duration,
	fetchBlockHash BlockHashFetcher,
	fetchBlockNumber BlockNumberFetcher,
	extract Extract,
	forecast ForecastResolution,
	monitors ...Monitor) *gameMonitor {
	return &gameMonitor{
		logger:           logger,
		clock:            cl,
		ctx:              ctx,
		done:             make(chan struct{}),
		metrics:          metrics,
		monitorInterval:  monitorInterval,
		gameWindow:       gameWindow,
		forecast:         forecast,
		monitors:         monitors,
		extract:          extract,
		fetchBlockNumber: fetchBlockNumber,
		fetchBlockHash:   fetchBlockHash,
	}
}

func (m *gameMonitor) monitorGames() error {
	start := m.clock.Now()
	blockNumber, err := m.fetchBlockNumber(m.ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch block number: %w", err)
	}
	m.logger.Debug("Fetched block number", "blockNumber", blockNumber)
	blockHash, err := m.fetchBlockHash(context.Background(), new(big.Int).SetUint64(blockNumber))
	if err != nil {
		return fmt.Errorf("failed to fetch block hash: %w", err)
	}
	minGameTimestamp := clock.MinCheckedTimestamp(m.clock, m.gameWindow)
	enrichedGames, ignored, failed, err := m.extract(m.ctx, blockHash, minGameTimestamp)
	if err != nil {
		return fmt.Errorf("failed to load games: %w", err)
	}
	m.forecast(enrichedGames, ignored, failed)
	for _, monitor := range m.monitors {
		monitor(enrichedGames)
	}
	timeTaken := m.clock.Since(start)
	m.metrics.RecordMonitorDuration(timeTaken)
	m.logger.Info("Completed monitoring update", "blockNumber", blockNumber, "blockHash", blockHash, "duration", timeTaken, "games", len(enrichedGames), "ignored", ignored, "failed", failed)
	return nil
}

func (m *gameMonitor) loop() {
	ticker := m.clock.NewTicker(m.monitorInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.Ch():
			if err := m.monitorGames(); err != nil {
				m.logger.Error("Failed to monitor games", "err", err)
			}
		case <-m.done:
			m.logger.Info("Stopping game monitor")
			return
		}
	}
}

func (m *gameMonitor) StartMonitoring() {
	// Setup the cancellation only if it's not already set.
	// This prevents overwriting the context and cancel function
	// if, for example, this function is called multiple times.
	if m.cancel == nil {
		ctx, cancel := context.WithCancel(m.ctx)
		m.ctx = ctx
		m.cancel = cancel
	}
	m.logger.Info("Starting game monitor")
	go m.loop()
}

func (m *gameMonitor) StopMonitoring() {
	m.logger.Info("Stopping game monitor")
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	close(m.done)
}
