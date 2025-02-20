package runner

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"

	contractMetrics "github.com/ethereum-optimism/optimism/op-challenger/game/fault/contracts/metrics"
	"github.com/ethereum-optimism/optimism/op-challenger/metrics"
	opmetrics "github.com/ethereum-optimism/optimism/op-service/metrics"
)

const Namespace = "op_challenger_runner"

type Metrics struct {
	ns       string
	registry *prometheus.Registry
	factory  opmetrics.Factory
	*contractMetrics.ContractMetrics
	*metrics.VmMetrics

	up                  prometheus.Gauge
	vmLastExecutionTime *prometheus.GaugeVec
	vmLastMemoryUsed    *prometheus.GaugeVec
	successTotal        *prometheus.CounterVec
	failuresTotal       *prometheus.CounterVec
	invalidTotal        *prometheus.CounterVec
}

var _ Metricer = (*Metrics)(nil)

// Metrics implementation must implement RegistryMetricer to allow the metrics server to work.
var _ opmetrics.RegistryMetricer = (*Metrics)(nil)

func NewMetrics(runConfigs []RunConfig) *Metrics {
	registry := opmetrics.NewRegistry()
	factory := opmetrics.With(registry)

	metrics := &Metrics{
		ns:       Namespace,
		registry: registry,
		factory:  factory,

		ContractMetrics: contractMetrics.MakeContractMetrics(Namespace, factory),
		VmMetrics:       metrics.NewVmMetrics(Namespace, factory),

		up: factory.NewGauge(prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "up",
			Help:      "The VM runner has started to run",
		}),
		vmLastExecutionTime: factory.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "vm_last_execution_time",
			Help:      "Time (in seconds) taken for the last execution of the fault proof VM",
		}, []string{"vm"}),
		vmLastMemoryUsed: factory.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "vm_last_memory_used",
			Help:      "Memory used (in bytes) for the last execution of the fault proof VM",
		}, []string{"vm"}),
		successTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: Namespace,
			Name:      "success_total",
			Help:      "Number of VM executions that successfully verified the output root",
		}, []string{"type"}),
		failuresTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: Namespace,
			Name:      "failures_total",
			Help:      "Number of failures to execute a VM",
		}, []string{"type"}),
		invalidTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: Namespace,
			Name:      "invalid_total",
			Help:      "Number of runs that determined the output root was invalid",
		}, []string{"type"}),
	}

	for _, runConfig := range runConfigs {
		metrics.successTotal.WithLabelValues(runConfig.Name).Add(0)
		metrics.failuresTotal.WithLabelValues(runConfig.Name).Add(0)
		metrics.invalidTotal.WithLabelValues(runConfig.Name).Add(0)
		metrics.RecordUp()
	}

	return metrics
}

func (m *Metrics) Registry() *prometheus.Registry {
	return m.registry
}

func (m *Metrics) RecordUp() {
	m.up.Set(1)
}

func (m *Metrics) RecordVmExecutionTime(vmType string, dur time.Duration) {
	val := dur.Seconds()
	m.VmMetrics.RecordVmExecutionTime(vmType, dur)
	m.vmLastExecutionTime.WithLabelValues(vmType).Set(val)
}

func (m *Metrics) RecordVmMemoryUsed(vmType string, memoryUsed uint64) {
	m.VmMetrics.RecordVmMemoryUsed(vmType, memoryUsed)
	m.vmLastMemoryUsed.WithLabelValues(vmType).Set(float64(memoryUsed))
}

func (m *Metrics) RecordSuccess(vmType string) {
	m.successTotal.WithLabelValues(vmType).Inc()
}

func (m *Metrics) RecordFailure(vmType string) {
	m.failuresTotal.WithLabelValues(vmType).Inc()
}

func (m *Metrics) RecordInvalid(vmType string) {
	m.invalidTotal.WithLabelValues(vmType).Inc()
}
