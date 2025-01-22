package backend

import (
	"time"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/sources/caching"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/cross"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/db/logs"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/processors"
)

type Metrics interface {
	CacheAdd(chainID eth.ChainID, label string, cacheSize int, evicted bool)
	CacheGet(chainID eth.ChainID, label string, hit bool)

	RecordDBEntryCount(chainID eth.ChainID, kind string, count int64)
	RecordDBSearchEntriesRead(chainID eth.ChainID, count int64)
	RecordDBLatency(chainID eth.ChainID, operation string, duration float64)
	RecordDBTruncation(chainID eth.ChainID)
	RecordDBSize(chainID eth.ChainID, sizeBytes int64)
	RecordDBInit(chainID eth.ChainID, success bool)

	RecordCrossChainOp(chainID eth.ChainID, success bool)
	RecordCrossChainLatency(chainID eth.ChainID, duration float64)
	RecordHazardCheck(chainID eth.ChainID)
	RecordHazardDetected(chainID eth.ChainID)
	RecordCycleDetection(chainID eth.ChainID, cycleFound bool)

	RecordWorkerProcessing(chainID eth.ChainID, eventType string)
	RecordWorkerLatency(chainID eth.ChainID, eventType string, duration float64)
	RecordWorkerError(chainID eth.ChainID, errorType string)
	RecordWorkerQueueSize(chainID eth.ChainID, size int)

	RecordBlockSealing(chainID eth.ChainID, success bool)
	RecordBlockVerification(chainID eth.ChainID, success bool)
	RecordBlockReorg(chainID eth.ChainID)
	RecordBlockProcessingLatency(chainID eth.ChainID, duration float64)
}

// chainMetrics is an adapter between the metrics API expected by clients that assume there's only a single chain
// and the actual metrics implementation which requires a chain ID to identify the source chain.
type chainMetrics struct {
	chainID  eth.ChainID
	delegate Metrics
}

func newChainMetrics(chainID eth.ChainID, delegate Metrics) *chainMetrics {
	return &chainMetrics{
		chainID:  chainID,
		delegate: delegate,
	}
}

func (c *chainMetrics) CacheAdd(label string, cacheSize int, evicted bool) {
	c.delegate.CacheAdd(c.chainID, label, cacheSize, evicted)
}

func (c *chainMetrics) CacheGet(label string, hit bool) {
	c.delegate.CacheGet(c.chainID, label, hit)
}

func (c *chainMetrics) RecordDBEntryCount(kind string, count int64) {
	c.delegate.RecordDBEntryCount(c.chainID, kind, count)
}

func (c *chainMetrics) RecordDBSearchEntriesRead(count int64) {
	c.delegate.RecordDBSearchEntriesRead(c.chainID, count)
}

func (c *chainMetrics) RecordDBLatency(operation string, duration time.Duration) {
	c.delegate.RecordDBLatency(c.chainID, operation, duration.Seconds())
}

func (c *chainMetrics) RecordDBTruncation() {
	c.delegate.RecordDBTruncation(c.chainID)
}

func (c *chainMetrics) RecordDBSize(sizeBytes int64) {
	c.delegate.RecordDBSize(c.chainID, sizeBytes)
}

func (c *chainMetrics) RecordDBInit(success bool) {
	c.delegate.RecordDBInit(c.chainID, success)
}

func (c *chainMetrics) RecordCrossChainOp(_ eth.ChainID, success bool) {
	c.delegate.RecordCrossChainOp(c.chainID, success)
}

func (c *chainMetrics) RecordCrossChainLatency(duration time.Duration) {
	c.delegate.RecordCrossChainLatency(c.chainID, duration.Seconds())
}

func (c *chainMetrics) RecordHazardCheck() {
	c.delegate.RecordHazardCheck(c.chainID)
}

func (c *chainMetrics) RecordHazardDetected(_ eth.ChainID) {
	c.delegate.RecordHazardDetected(c.chainID)
}

func (c *chainMetrics) RecordCycleDetection(cycleFound bool) {
	c.delegate.RecordCycleDetection(c.chainID, cycleFound)
}

func (c *chainMetrics) RecordWorkerProcessing(eventType string) {
	c.delegate.RecordWorkerProcessing(c.chainID, eventType)
}

func (c *chainMetrics) RecordWorkerLatency(eventType string, duration time.Duration) {
	c.delegate.RecordWorkerLatency(c.chainID, eventType, duration.Seconds())
}

func (c *chainMetrics) RecordWorkerError(errorType string) {
	c.delegate.RecordWorkerError(c.chainID, errorType)
}

func (c *chainMetrics) RecordWorkerQueueSize(size int) {
	c.delegate.RecordWorkerQueueSize(c.chainID, size)
}

func (c *chainMetrics) RecordBlockSealing(success bool) {
	c.delegate.RecordBlockSealing(c.chainID, success)
}

func (c *chainMetrics) RecordBlockVerification() {
	c.delegate.RecordBlockVerification(c.chainID, true)
}

func (c *chainMetrics) RecordBlockReorg() {
	c.delegate.RecordBlockReorg(c.chainID)
}

func (c *chainMetrics) RecordBlockProcessingLatency(duration time.Duration) {
	c.delegate.RecordBlockProcessingLatency(c.chainID, duration.Seconds())
}

var _ caching.Metrics = (*chainMetrics)(nil)
var _ logs.Metrics = (*chainMetrics)(nil)
var _ cross.ProcessorMetrics = (*chainMetrics)(nil)
var _ processors.ProcessorMetrics = (*chainMetrics)(nil)
