package mon

import (
	"context"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-challenger/game/types"
	"github.com/ethereum-optimism/optimism/op-dispute-mon/metrics"
	monTypes "github.com/ethereum-optimism/optimism/op-dispute-mon/mon/types"
	"github.com/ethereum-optimism/optimism/op-service/clock"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"
)

var (
	mockErr = errors.New("mock error")
)

func TestMonitor_MonitorGames(t *testing.T) {
	t.Parallel()

	t.Run("FailedFetchBlocknumber", func(t *testing.T) {
		monitor, _, _, _ := setupMonitorTest(t)
		boom := errors.New("boom")
		monitor.fetchBlockNumber = func(ctx context.Context) (uint64, error) {
			return 0, boom
		}
		err := monitor.monitorGames()
		require.ErrorIs(t, err, boom)
	})

	t.Run("FailedFetchBlockHash", func(t *testing.T) {
		monitor, _, _, _ := setupMonitorTest(t)
		boom := errors.New("boom")
		monitor.fetchBlockHash = func(ctx context.Context, number *big.Int) (common.Hash, error) {
			return common.Hash{}, boom
		}
		err := monitor.monitorGames()
		require.ErrorIs(t, err, boom)
	})

	t.Run("MonitorsWithNoGames", func(t *testing.T) {
		monitor, factory, forecast, monitors := setupMonitorTest(t)
		factory.games = []*monTypes.EnrichedGameData{}
		err := monitor.monitorGames()
		require.NoError(t, err)
		require.Equal(t, 1, forecast.calls)
		for _, m := range monitors {
			require.Equal(t, 1, m.calls)
		}
	})

	t.Run("MonitorsMultipleGames", func(t *testing.T) {
		monitor, factory, forecast, monitors := setupMonitorTest(t)
		factory.games = []*monTypes.EnrichedGameData{{}, {}, {}}
		err := monitor.monitorGames()
		require.NoError(t, err)
		require.Equal(t, 1, forecast.calls)
		for _, m := range monitors {
			require.Equal(t, 1, m.calls)
		}
	})
}

func TestMonitor_StartMonitoring(t *testing.T) {
	t.Run("MonitorsGames", func(t *testing.T) {
		addr1 := common.Address{0xaa}
		addr2 := common.Address{0xbb}
		monitor, factory, forecaster, _ := setupMonitorTest(t)
		factory.games = []*monTypes.EnrichedGameData{newEnrichedGameData(addr1, 9999), newEnrichedGameData(addr2, 9999)}
		factory.maxSuccess = len(factory.games) // Only allow two successful fetches

		monitor.StartMonitoring()
		require.Eventually(t, func() bool {
			return forecaster.calls >= 2
		}, time.Second, 50*time.Millisecond)
		monitor.StopMonitoring()
		require.Equal(t, len(factory.games), forecaster.calls) // Each game's status is recorded twice
	})

	t.Run("FailsToFetchGames", func(t *testing.T) {
		monitor, factory, forecaster, _ := setupMonitorTest(t)
		factory.fetchErr = errors.New("boom")

		monitor.StartMonitoring()
		require.Eventually(t, func() bool {
			return factory.calls > 0
		}, time.Second, 50*time.Millisecond)
		monitor.StopMonitoring()
		require.Equal(t, 0, forecaster.calls)
	})
}

func newEnrichedGameData(proxy common.Address, timestamp uint64) *monTypes.EnrichedGameData {
	return &monTypes.EnrichedGameData{
		GameMetadata: types.GameMetadata{
			Proxy:     proxy,
			Timestamp: timestamp,
		},
		Status: types.GameStatusInProgress,
	}
}

func setupMonitorTest(t *testing.T) (*gameMonitor, *mockExtractor, *mockForecast, []*mockMonitor) {
	logger := testlog.Logger(t, log.LvlDebug)
	fetchBlockNum := func(ctx context.Context) (uint64, error) {
		return 1, nil
	}
	fetchBlockHash := func(ctx context.Context, number *big.Int) (common.Hash, error) {
		return common.Hash{}, nil
	}
	monitorInterval := 100 * time.Millisecond
	cl := clock.NewAdvancingClock(10 * time.Millisecond)
	cl.Start()
	extractor := &mockExtractor{}
	forecast := &mockForecast{}
	monitor1 := &mockMonitor{}
	monitor2 := &mockMonitor{}
	monitor3 := &mockMonitor{}
	monitor := newGameMonitor(context.Background(), logger, cl, metrics.NoopMetrics, monitorInterval, 10*time.Second, fetchBlockHash, fetchBlockNum,
		extractor.Extract, forecast.Forecast, monitor1.Check, monitor2.Check, monitor3.Check)
	return monitor, extractor, forecast, []*mockMonitor{monitor1, monitor2, monitor3}
}

type mockMonitor struct {
	calls int
}

func (m *mockMonitor) Check(games []*monTypes.EnrichedGameData) {
	m.calls++
}

type mockForecast struct {
	calls int
}

func (m *mockForecast) Forecast(_ []*monTypes.EnrichedGameData, _, _ int) {
	m.calls++
}

type mockExtractor struct {
	fetchErr     error
	calls        int
	maxSuccess   int
	games        []*monTypes.EnrichedGameData
	ignoredCount int
	failedCount  int
}

func (m *mockExtractor) Extract(
	_ context.Context,
	_ common.Hash,
	_ uint64,
) ([]*monTypes.EnrichedGameData, int, int, error) {
	m.calls++
	if m.fetchErr != nil {
		return nil, 0, 0, m.fetchErr
	}
	if m.calls > m.maxSuccess && m.maxSuccess != 0 {
		return nil, 0, 0, mockErr
	}
	return m.games, m.ignoredCount, m.failedCount, nil
}
