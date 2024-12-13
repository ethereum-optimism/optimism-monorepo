package metrics

import (
	"github.com/ethereum/go-ethereum/core/types"
)

type TestMetrics struct {
	noopMetrics
	PendingBlocksBytesCurrent float64
}

var _ Metricer = new(TestMetrics)

func (m *TestMetrics) RecordL2BlockInPendingQueue(block *types.Block) {
	_, rawSize := estimateBatchSize(block)
	m.PendingBlocksBytesCurrent += float64(rawSize)

}
func (m *TestMetrics) RecordL2BlockInChannel(block *types.Block) {
	_, rawSize := estimateBatchSize(block)
	m.PendingBlocksBytesCurrent -= float64(rawSize)
}
