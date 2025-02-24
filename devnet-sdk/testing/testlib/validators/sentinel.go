package validators

import "sync/atomic"

var (
	sentinelID atomic.Uint64
)

type sentinelMarker struct {
	id uint64
}

func newSentinelMarker() *sentinelMarker {
	return &sentinelMarker{
		id: sentinelID.Add(1),
	}
}
