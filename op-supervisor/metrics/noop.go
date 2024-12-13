package metrics

import (
	opmetrics "github.com/ethereum-optimism/optimism/op-service/metrics"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

type noopMetrics struct {
	opmetrics.NoopRPCMetrics
}

var NoopMetrics Metricer = new(noopMetrics)

func (*noopMetrics) Document() []opmetrics.DocumentedMetric { return nil }

func (*noopMetrics) RecordInfo(version string) {}
func (*noopMetrics) RecordUp()                 {}

func (m *noopMetrics) CacheAdd(_ types.ChainID, _ string, _ int, _ bool) {}
func (m *noopMetrics) CacheGet(_ types.ChainID, _ string, _ bool)        {}

func (m *noopMetrics) RecordDBEntryCount(_ types.ChainID, _ string, _ int64) {}
func (m *noopMetrics) RecordDBSearchEntriesRead(_ types.ChainID, _ int64)    {}
