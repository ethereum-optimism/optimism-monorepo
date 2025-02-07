package metrics

import (
	"github.com/ethereum-optimism/optimism/op-node/rollup/event"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	opmetrics "github.com/ethereum-optimism/optimism/op-service/metrics"
)

type noopMetrics struct {
	opmetrics.NoopRPCMetrics
	event.NoopMetrics
}

var NoopMetrics Metricer = new(noopMetrics)

func (*noopMetrics) Document() []opmetrics.DocumentedMetric { return nil }

func (*noopMetrics) RecordInfo(version string) {}
func (*noopMetrics) RecordUp()                 {}

func (m *noopMetrics) RecordCrossUnsafeRef(_ eth.ChainID, _ eth.BlockRef) {}
func (m *noopMetrics) RecordCrossSafeRef(_ eth.ChainID, _ eth.BlockRef)   {}

func (m *noopMetrics) CacheAdd(_ eth.ChainID, _ string, _ int, _ bool) {}
func (m *noopMetrics) CacheGet(_ eth.ChainID, _ string, _ bool)        {}

func (m *noopMetrics) RecordDBEntryCount(_ eth.ChainID, _ string, _ int64) {}
func (m *noopMetrics) RecordDBSearchEntriesRead(_ eth.ChainID, _ int64)    {}
