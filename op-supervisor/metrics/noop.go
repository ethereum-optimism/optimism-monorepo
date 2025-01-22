package metrics

import (
	"github.com/ethereum-optimism/optimism/op-service/eth"
	opmetrics "github.com/ethereum-optimism/optimism/op-service/metrics"
)

type noopMetrics struct {
	opmetrics.NoopRPCMetrics
}

var NoopMetrics Metricer = new(noopMetrics)

func (*noopMetrics) RecordInfo(version string) {}
func (*noopMetrics) RecordUp()                 {}

func (m *noopMetrics) CacheAdd(_ eth.ChainID, label string, size int, evicted bool) {}
func (m *noopMetrics) CacheGet(_ eth.ChainID, label string, hit bool)               {}

func (m *noopMetrics) RecordDBEntryCount(_ eth.ChainID, kind string, count int64) {}
func (m *noopMetrics) RecordDBSearchEntriesRead(_ eth.ChainID, count int64)       {}

func (*noopMetrics) Document() []opmetrics.DocumentedMetric { return nil }

func (*noopMetrics) RecordRPCClientRequest(method string) func(error) {
	return func(error) {}
}
func (*noopMetrics) RecordRPCClientResponse(method string, err error)               {}
func (*noopMetrics) RecordRPCClientRequestDuration(method string, duration float64) {}

func (*noopMetrics) RecordBlockSealing(_ eth.ChainID, success bool)               {}
func (*noopMetrics) RecordBlockVerification(_ eth.ChainID, success bool)          {}
func (*noopMetrics) RecordBlockReorg(_ eth.ChainID)                               {}
func (*noopMetrics) RecordBlockProcessingLatency(_ eth.ChainID, duration float64) {}

func (*noopMetrics) RecordDBLatency(_ eth.ChainID, operation string, duration float64) {}
func (*noopMetrics) RecordDBTruncation(_ eth.ChainID)                                  {}
func (*noopMetrics) RecordDBSize(_ eth.ChainID, sizeBytes int64)                       {}
func (*noopMetrics) RecordDBInit(_ eth.ChainID, success bool)                          {}

func (*noopMetrics) RecordCrossChainOp(_ eth.ChainID, success bool)          {}
func (*noopMetrics) RecordCrossChainLatency(_ eth.ChainID, duration float64) {}
func (*noopMetrics) RecordHazardCheck(_ eth.ChainID)                         {}
func (*noopMetrics) RecordHazardDetected(_ eth.ChainID)                      {}
func (*noopMetrics) RecordCycleDetection(_ eth.ChainID, cycleFound bool)     {}

func (*noopMetrics) RecordWorkerProcessing(_ eth.ChainID, eventType string)                {}
func (*noopMetrics) RecordWorkerLatency(_ eth.ChainID, eventType string, duration float64) {}
func (*noopMetrics) RecordWorkerError(_ eth.ChainID, errorType string)                     {}
func (*noopMetrics) RecordWorkerQueueSize(_ eth.ChainID, size int)                         {}
