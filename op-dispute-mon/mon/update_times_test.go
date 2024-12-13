package mon

import (
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-dispute-mon/mon/types"
	"github.com/ethereum-optimism/optimism/op-service/clock"
	"github.com/stretchr/testify/require"
)

func TestUpdateTimeMonitor_NoGames(t *testing.T) {
	m := &mockUpdateTimeMetrics{}
	cl := clock.NewDeterministicClock(time.UnixMilli(45892))
	monitor := NewUpdateTimeMonitor(cl, m)
	monitor.CheckUpdateTimes(nil)
	require.Equal(t, cl.Now(), m.oldestUpdateTime)

	cl.AdvanceTime(time.Minute)
	monitor.CheckUpdateTimes([]*types.EnrichedGameData{})
	require.Equal(t, cl.Now(), m.oldestUpdateTime)
}

func TestUpdateTimeMonitor_ReportsOldestUpdateTime(t *testing.T) {
	m := &mockUpdateTimeMetrics{}
	cl := clock.NewDeterministicClock(time.UnixMilli(45892))
	monitor := NewUpdateTimeMonitor(cl, m)
	monitor.CheckUpdateTimes([]*types.EnrichedGameData{
		{LastUpdateTime: time.UnixMilli(4)},
		{LastUpdateTime: time.UnixMilli(3)},
		{LastUpdateTime: time.UnixMilli(7)},
		{LastUpdateTime: time.UnixMilli(9)},
	})
	require.Equal(t, time.UnixMilli(3), m.oldestUpdateTime)
}

type mockUpdateTimeMetrics struct {
	oldestUpdateTime time.Time
}

func (m *mockUpdateTimeMetrics) RecordOldestGameUpdateTime(t time.Time) {
	m.oldestUpdateTime = t
}
