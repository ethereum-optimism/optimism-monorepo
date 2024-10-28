package mon

import (
	"time"

	"github.com/ethereum-optimism/optimism/op-dispute-mon/mon/types"
	"github.com/ethereum-optimism/optimism/op-service/clock"
)

type UpdateTimeMetrics interface {
	RecordOldestGameUpdateTime(t time.Time)
}

type UpdateTimeMonitor struct {
	metrics UpdateTimeMetrics
	clock   clock.Clock
}

func NewUpdateTimeMonitor(cl clock.Clock, metrics UpdateTimeMetrics) *UpdateTimeMonitor {
	return &UpdateTimeMonitor{clock: cl, metrics: metrics}
}

func (m *UpdateTimeMonitor) CheckUpdateTimes(games []*types.EnrichedGameData) {
	// Report the current time if there are no games
	// Otherwise the last update time would drop to 0 when there are no games, making it appear there were errors
	earliest := m.clock.Now()

	for _, game := range games {
		if game.LastUpdateTime.Before(earliest) {
			earliest = game.LastUpdateTime
		}
	}
	m.metrics.RecordOldestGameUpdateTime(earliest)
}
