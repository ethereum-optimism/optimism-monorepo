package metrics

import (
	"strconv"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/ethereum-optimism/optimism/op-service/eth"

	opmetrics "github.com/ethereum-optimism/optimism/op-service/metrics"
)

const Namespace = "op_supervisor"

type Metricer interface {
	RecordInfo(version string)
	RecordUp()

	opmetrics.RPCMetricer

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
	RecordWorkerError(chainID eth.ChainID, errorType string)
	RecordWorkerQueueSize(chainID eth.ChainID, size int)

	RecordBlockSealing(chainID eth.ChainID, success bool)
	RecordBlockVerification(chainID eth.ChainID, success bool)
	RecordBlockReorg(chainID eth.ChainID)

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

	DBLatencyVec    *prometheus.HistogramVec
	DBTruncationVec *prometheus.CounterVec
	DBSizeVec       *prometheus.GaugeVec
	DBInitVec       *prometheus.CounterVec

	CrossChainOpsVec     *prometheus.CounterVec
	CrossChainLatencyVec *prometheus.HistogramVec
	HazardChecksVec      *prometheus.CounterVec
	HazardsDetectedVec   *prometheus.CounterVec
	CycleDetectionVec    *prometheus.CounterVec

	WorkerProcessingVec *prometheus.CounterVec
	WorkerLatencyVec    *prometheus.HistogramVec
	WorkerErrorsVec     *prometheus.CounterVec
	WorkerQueueSizeVec  *prometheus.GaugeVec

	BlockSealingVec      *prometheus.CounterVec
	BlockVerificationVec *prometheus.CounterVec
	BlockReorgVec        *prometheus.CounterVec
	BlockLatencyVec      *prometheus.HistogramVec

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

		DBLatencyVec: factory.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: ns,
			Name:      "db_operation_latency_seconds",
			Help:      "Latency of database operations",
			Buckets:   []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		}, []string{
			"chain",
			"operation",
		}),
		DBTruncationVec: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: ns,
			Name:      "db_truncations_total",
			Help:      "Number of database truncations performed",
		}, []string{
			"chain",
		}),
		DBSizeVec: factory.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns,
			Name:      "db_size_bytes",
			Help:      "Current size of the database in bytes",
		}, []string{
			"chain",
		}),
		DBInitVec: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: ns,
			Name:      "db_init_total",
			Help:      "Number of database initializations",
		}, []string{
			"chain",
			"success",
		}),

		CrossChainOpsVec: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: ns,
			Name:      "cross_chain_ops_total",
			Help:      "Number of cross-chain operations",
		}, []string{
			"chain",
			"success",
		}),
		CrossChainLatencyVec: factory.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: ns,
			Name:      "cross_chain_latency_seconds",
			Help:      "Latency of cross-chain operations",
			Buckets:   []float64{.1, .5, 1, 2.5, 5, 10, 30, 60, 120, 300},
		}, []string{
			"chain",
		}),
		HazardChecksVec: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: ns,
			Name:      "hazard_checks_total",
			Help:      "Number of hazard checks performed",
		}, []string{
			"chain",
		}),
		HazardsDetectedVec: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: ns,
			Name:      "hazards_detected_total",
			Help:      "Number of hazards detected",
		}, []string{
			"chain",
		}),
		CycleDetectionVec: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: ns,
			Name:      "cycle_detection_total",
			Help:      "Number of cycle detections performed",
		}, []string{
			"chain",
			"cycle_found",
		}),

		WorkerProcessingVec: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: ns,
			Name:      "worker_events_total",
			Help:      "Number of events processed by workers",
		}, []string{
			"chain",
			"event_type",
		}),
		WorkerLatencyVec: factory.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: ns,
			Name:      "worker_processing_latency_seconds",
			Help:      "Latency of worker event processing",
			Buckets:   []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
		}, []string{
			"chain",
			"event_type",
		}),
		WorkerErrorsVec: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: ns,
			Name:      "worker_errors_total",
			Help:      "Number of worker errors by type",
		}, []string{
			"chain",
			"error_type",
		}),
		WorkerQueueSizeVec: factory.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns,
			Name:      "worker_queue_size",
			Help:      "Current size of worker event queues",
		}, []string{
			"chain",
		}),

		BlockSealingVec: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: ns,
			Name:      "block_sealing_total",
			Help:      "Number of block sealing operations",
		}, []string{
			"chain",
			"success",
		}),
		BlockVerificationVec: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: ns,
			Name:      "block_verification_total",
			Help:      "Number of block verifications",
		}, []string{
			"chain",
			"success",
		}),
		BlockReorgVec: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: ns,
			Name:      "block_reorgs_total",
			Help:      "Number of block reorganizations",
		}, []string{
			"chain",
		}),
		BlockLatencyVec: factory.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: ns,
			Name:      "block_processing_latency_seconds",
			Help:      "Latency of block processing",
			Buckets:   []float64{.01, .05, .1, .25, .5, 1, 2.5, 5, 10},
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

func (m *Metrics) CacheAdd(chainID eth.ChainID, label string, cacheSize int, evicted bool) {
	chain := chainIDLabel(chainID)
	m.CacheSizeVec.WithLabelValues(chain, label).Set(float64(cacheSize))
	if evicted {
		m.CacheAddVec.WithLabelValues(chain, label, "true").Inc()
	} else {
		m.CacheAddVec.WithLabelValues(chain, label, "false").Inc()
	}
}

func (m *Metrics) CacheGet(chainID eth.ChainID, label string, hit bool) {
	chain := chainIDLabel(chainID)
	if hit {
		m.CacheGetVec.WithLabelValues(chain, label, "true").Inc()
	} else {
		m.CacheGetVec.WithLabelValues(chain, label, "false").Inc()
	}
}

func (m *Metrics) RecordDBEntryCount(chainID eth.ChainID, kind string, count int64) {
	m.DBEntryCountVec.WithLabelValues(chainIDLabel(chainID), kind).Set(float64(count))
}

func (m *Metrics) RecordDBSearchEntriesRead(chainID eth.ChainID, count int64) {
	m.DBSearchEntriesReadVec.WithLabelValues(chainIDLabel(chainID)).Observe(float64(count))
}

func (m *Metrics) RecordDBLatency(chainID eth.ChainID, operation string, duration float64) {
	m.DBLatencyVec.WithLabelValues(chainIDLabel(chainID), operation).Observe(duration)
}

func (m *Metrics) RecordDBTruncation(chainID eth.ChainID) {
	m.DBTruncationVec.WithLabelValues(chainIDLabel(chainID)).Inc()
}

func (m *Metrics) RecordDBSize(chainID eth.ChainID, sizeBytes int64) {
	m.DBSizeVec.WithLabelValues(chainIDLabel(chainID)).Set(float64(sizeBytes))
}

func (m *Metrics) RecordDBInit(chainID eth.ChainID, success bool) {
	m.DBInitVec.WithLabelValues(chainIDLabel(chainID), strconv.FormatBool(success)).Inc()
}

func (m *Metrics) RecordCrossChainOp(chainID eth.ChainID, success bool) {
	m.CrossChainOpsVec.WithLabelValues(chainIDLabel(chainID), strconv.FormatBool(success)).Inc()
}

func (m *Metrics) RecordCrossChainLatency(chainID eth.ChainID, duration float64) {
	m.CrossChainLatencyVec.WithLabelValues(chainIDLabel(chainID)).Observe(duration)
}

func (m *Metrics) RecordHazardCheck(chainID eth.ChainID) {
	m.HazardChecksVec.WithLabelValues(chainIDLabel(chainID)).Inc()
}

func (m *Metrics) RecordHazardDetected(chainID eth.ChainID) {
	m.HazardsDetectedVec.WithLabelValues(chainIDLabel(chainID)).Inc()
}

func (m *Metrics) RecordCycleDetection(chainID eth.ChainID, cycleFound bool) {
	m.CycleDetectionVec.WithLabelValues(chainIDLabel(chainID), strconv.FormatBool(cycleFound)).Inc()
}

func (m *Metrics) RecordWorkerProcessing(chainID eth.ChainID, eventType string) {
	m.WorkerProcessingVec.WithLabelValues(chainIDLabel(chainID), eventType).Inc()
}

func (m *Metrics) RecordWorkerLatency(chainID eth.ChainID, eventType string, duration float64) {
	m.WorkerLatencyVec.WithLabelValues(chainIDLabel(chainID), eventType).Observe(duration)
}

func (m *Metrics) RecordWorkerError(chainID eth.ChainID, errorType string) {
	m.WorkerErrorsVec.WithLabelValues(chainIDLabel(chainID), errorType).Inc()
}

func (m *Metrics) RecordWorkerQueueSize(chainID eth.ChainID, size int) {
	m.WorkerQueueSizeVec.WithLabelValues(chainIDLabel(chainID)).Set(float64(size))
}

func (m *Metrics) RecordBlockSealing(chainID eth.ChainID, success bool) {
	m.BlockSealingVec.WithLabelValues(chainIDLabel(chainID), strconv.FormatBool(success)).Inc()
}

func (m *Metrics) RecordBlockVerification(chainID eth.ChainID, success bool) {
	m.BlockVerificationVec.WithLabelValues(chainIDLabel(chainID), strconv.FormatBool(success)).Inc()
}

func (m *Metrics) RecordBlockReorg(chainID eth.ChainID) {
	m.BlockReorgVec.WithLabelValues(chainIDLabel(chainID)).Inc()
}

func (m *Metrics) RecordBlockProcessingLatency(chainID eth.ChainID, duration float64) {
	m.BlockLatencyVec.WithLabelValues(chainIDLabel(chainID)).Observe(duration)
}

func chainIDLabel(chainID eth.ChainID) string {
	return chainID.String()
}
