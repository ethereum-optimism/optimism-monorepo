package metrics

import (
	"github.com/prometheus/client_golang/prometheus"

	opmetrics "github.com/ethereum-optimism/optimism/op-service/metrics"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

const Namespace = "op_supervisor"

type Metricer interface {
	RecordInfo(version string)
	RecordUp()

	opmetrics.RPCMetricer

	CacheAdd(chainID types.ChainID, label string, cacheSize int, evicted bool)
	CacheGet(chainID types.ChainID, label string, hit bool)

	RecordDBEntryCount(chainID types.ChainID, kind string, count int64)
	RecordDBSearchEntriesRead(chainID types.ChainID, count int64)

	Document() []opmetrics.DocumentedMetric
}

type Metrics struct {
	ns       string
	registry *prometheus.Registry
	factory  opmetrics.Factory

	opmetrics.RPCMetrics

	CacheSizeVec *prometheus.GaugeVec
	CacheGetVec  *prometheus.CounterVec
	CacheAddVec  *prometheus.CounterVec

	DBEntryCountVec        *prometheus.GaugeVec
	DBSearchEntriesReadVec *prometheus.HistogramVec

	info prometheus.GaugeVec
	up   prometheus.Gauge
}

var _ Metricer = (*Metrics)(nil)

// implements the Registry getter, for metrics HTTP server to hook into
var _ opmetrics.RegistryMetricer = (*Metrics)(nil)

func NewMetrics(procName string) *Metrics {
	if procName == "" {
		procName = "default"
	}
	ns := Namespace + "_" + procName

	registry := opmetrics.NewRegistry()
	factory := opmetrics.With(registry)

	return &Metrics{
		ns:       ns,
		registry: registry,
		factory:  factory,

		RPCMetrics: opmetrics.MakeRPCMetrics(ns, factory),

		info: *factory.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns,
			Name:      "info",
			Help:      "Pseudo-metric tracking version and config info",
		}, []string{
			"version",
		}),
		up: factory.NewGauge(prometheus.GaugeOpts{
			Namespace: ns,
			Name:      "up",
			Help:      "1 if the op-supervisor has finished starting up",
		}),

		CacheSizeVec: factory.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns,
			Name:      "source_rpc_cache_size",
			Help:      "Source rpc cache cache size",
		}, []string{
			"chain",
			"type",
		}),
		CacheGetVec: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: ns,
			Name:      "source_rpc_cache_get",
			Help:      "Source rpc cache lookups, hitting or not",
		}, []string{
			"chain",
			"type",
			"hit",
		}),
		CacheAddVec: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: ns,
			Name:      "source_rpc_cache_add",
			Help:      "Source rpc cache additions, evicting previous values or not",
		}, []string{
			"chain",
			"type",
			"evicted",
		}),

		DBEntryCountVec: factory.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns,
			Name:      "logdb_entries_current",
			Help:      "Current number of entries in the database of specified kind and chain ID",
		}, []string{
			"chain",
			"kind",
		}),
		DBSearchEntriesReadVec: factory.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: ns,
			Name:      "logdb_search_entries_read",
			Help:      "Entries read per search of the log database",
			Buckets:   []float64{1, 2, 5, 10, 100, 200, 256},
		}, []string{
			"chain",
		}),
	}
}

func (m *Metrics) Registry() *prometheus.Registry {
	return m.registry
}

func (m *Metrics) Document() []opmetrics.DocumentedMetric {
	return m.factory.Document()
}

// RecordInfo sets a pseudo-metric that contains versioning and config info for the op-supervisor.
func (m *Metrics) RecordInfo(version string) {
	m.info.WithLabelValues(version).Set(1)
}

// RecordUp sets the up metric to 1.
func (m *Metrics) RecordUp() {
	prometheus.MustRegister()
	m.up.Set(1)
}

func (m *Metrics) CacheAdd(chainID types.ChainID, label string, cacheSize int, evicted bool) {
	chain := chainIDLabel(chainID)
	m.CacheSizeVec.WithLabelValues(chain, label).Set(float64(cacheSize))
	if evicted {
		m.CacheAddVec.WithLabelValues(chain, label, "true").Inc()
	} else {
		m.CacheAddVec.WithLabelValues(chain, label, "false").Inc()
	}
}

func (m *Metrics) CacheGet(chainID types.ChainID, label string, hit bool) {
	chain := chainIDLabel(chainID)
	if hit {
		m.CacheGetVec.WithLabelValues(chain, label, "true").Inc()
	} else {
		m.CacheGetVec.WithLabelValues(chain, label, "false").Inc()
	}
}

func (m *Metrics) RecordDBEntryCount(chainID types.ChainID, kind string, count int64) {
	m.DBEntryCountVec.WithLabelValues(chainIDLabel(chainID), kind).Set(float64(count))
}

func (m *Metrics) RecordDBSearchEntriesRead(chainID types.ChainID, count int64) {
	m.DBSearchEntriesReadVec.WithLabelValues(chainIDLabel(chainID)).Observe(float64(count))
}

func chainIDLabel(chainID types.ChainID) string {
	return chainID.String()
}
