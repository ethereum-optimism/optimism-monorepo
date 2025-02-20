package backend

import (
	"github.com/ethereum-optimism/optimism/op-node/rollup/event"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/sources/caching"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/db/logs"
)

type Metrics interface {
	CacheAdd(chainID eth.ChainID, label string, cacheSize int, evicted bool)
	CacheGet(chainID eth.ChainID, label string, hit bool)

	RecordCrossUnsafeRef(chainID eth.ChainID, ref eth.BlockRef)
	RecordCrossSafeRef(chainID eth.ChainID, ref eth.BlockRef)

	RecordDBEntryCount(chainID eth.ChainID, kind string, count int64)
	RecordDBSearchEntriesRead(chainID eth.ChainID, count int64)

	event.Metrics
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

func (c *chainMetrics) RecordCrossUnsafeRef(ref eth.BlockRef) {
	c.delegate.RecordCrossUnsafeRef(c.chainID, ref)
}

func (c *chainMetrics) RecordCrossSafeRef(ref eth.BlockRef) {
	c.delegate.RecordCrossSafeRef(c.chainID, ref)
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

var _ caching.Metrics = (*chainMetrics)(nil)
var _ logs.Metrics = (*chainMetrics)(nil)
