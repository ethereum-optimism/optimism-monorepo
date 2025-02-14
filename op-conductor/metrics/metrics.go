package metrics

import (
	"strconv"

	"github.com/ethereum-optimism/optimism/op-service/httputil"
	opmetrics "github.com/ethereum-optimism/optimism/op-service/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

const Namespace = "op_conductor"

type Metricer interface {
	RecordInfo(version string)
	RecordUp()
	RecordStateChange(leader bool, healthy bool, active bool)
	RecordLeaderTransfer(success bool)
	RecordStartSequencer(success bool)
	RecordStopSequencer(success bool)
	RecordHealthCheck(success bool, err error)
	RecordLoopExecutionTime(duration float64)
}

// Metrics implementation must implement RegistryMetricer to allow the metrics server to work.
var _ opmetrics.RegistryMetricer = (*Metrics)(nil)

type Metrics struct {
	ns       string
	registry *prometheus.Registry
	factory  opmetrics.Factory

	info prometheus.GaugeVec
	up   prometheus.Gauge

	healthChecks    *prometheus.CounterVec
	leaderTransfers *prometheus.CounterVec
	sequencerStarts *prometheus.CounterVec
	sequencerStops  *prometheus.CounterVec
	stateChanges    *prometheus.CounterVec

	loopExecutionTime prometheus.Histogram
}

func (m *Metrics) Registry() *prometheus.Registry {
	return m.registry
}

var _ Metricer = (*Metrics)(nil)

func NewMetrics() *Metrics {
	registry := opmetrics.NewRegistry()
	factory := opmetrics.With(registry)

	return &Metrics{
		ns:       Namespace,
		registry: registry,
		factory:  factory,

		info: *factory.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "info",
			Help:      "Pseudo-metric tracking version and config info",
		}, []string{
			"version",
		}),
		up: factory.NewGauge(prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "up",
			Help:      "1 if the op-conductor has finished starting up",
		}),
		healthChecks: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: Namespace,
			Name:      "healthchecks_count",
			Help:      "Number of healthchecks",
		}, []string{"success", "error"}),
		leaderTransfers: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: Namespace,
			Name:      "leader_transfers_count",
			Help:      "Number of leader transfers",
		}, []string{"success"}),
		sequencerStarts: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: Namespace,
			Name:      "sequencer_starts_count",
			Help:      "Number of sequencer starts",
		}, []string{"success"}),
		sequencerStops: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: Namespace,
			Name:      "sequencer_stops_count",
			Help:      "Number of sequencer stops",
		}, []string{"success"}),
		stateChanges: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: Namespace,
			Name:      "state_changes_count",
			Help:      "Number of state changes",
		}, []string{
			"leader",
			"healthy",
			"active",
		}),
		loopExecutionTime: factory.NewHistogram(prometheus.HistogramOpts{
			Namespace: Namespace,
			Name:      "loop_execution_time",
			Help:      "Time (in seconds) to execute conductor loop iteration",
			Buckets:   []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		}),
	}
}

func (m *Metrics) Start(host string, port int) (*httputil.HTTPServer, error) {
	return opmetrics.StartServer(m.registry, host, port)
}

// RecordInfo sets a pseudo-metric that contains versioning and
// config info for the op-proposer.
func (m *Metrics) RecordInfo(version string) {
	m.info.WithLabelValues(version).Set(1)
}

// RecordUp sets the up metric to 1.
func (m *Metrics) RecordUp() {
	m.up.Set(1)
}

// RecordHealthCheck increments the healthChecks counter.
func (m *Metrics) RecordHealthCheck(success bool, err error) {
	errStr := ""
	if err != nil {
		errStr = err.Error()
	}
	m.healthChecks.WithLabelValues(strconv.FormatBool(success), errStr).Inc()
}

// RecordLeaderTransfer increments the leaderTransfers counter.
func (m *Metrics) RecordLeaderTransfer(success bool) {
	m.leaderTransfers.WithLabelValues(strconv.FormatBool(success)).Inc()
}

// RecordStateChange increments the stateChanges counter.
func (m *Metrics) RecordStateChange(leader bool, healthy bool, active bool) {
	m.stateChanges.WithLabelValues(strconv.FormatBool(leader), strconv.FormatBool(healthy), strconv.FormatBool(active)).Inc()
}

// RecordStartSequencer increments the sequencerStarts counter.
func (m *Metrics) RecordStartSequencer(success bool) {
	m.sequencerStarts.WithLabelValues(strconv.FormatBool(success)).Inc()
}

// RecordStopSequencer increments the sequencerStops counter.
func (m *Metrics) RecordStopSequencer(success bool) {
	m.sequencerStops.WithLabelValues(strconv.FormatBool(success)).Inc()
}

// RecordLoopExecutionTime records the time it took to execute the conductor loop.
func (m *Metrics) RecordLoopExecutionTime(duration float64) {
	m.loopExecutionTime.Observe(duration)
}
