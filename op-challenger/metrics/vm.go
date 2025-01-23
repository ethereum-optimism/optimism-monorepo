package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/ethereum-optimism/optimism/op-service/metrics"
)

type VmMetricer interface {
	RecordVmExecutionTime(vmType string, t time.Duration)
	RecordVmMemoryUsed(vmType string, memoryUsed uint64)
	RecordVmRmwSuccessCount(vmType string, val uint64)
	RecordVmSteps(vmType string, val uint64)
	RecordVmRmwFailCount(vmType string, val uint64)
	RecordVmMaxStepsBetweenLLAndSC(vmType string, val uint64)
	RecordVmReservationInvalidationCount(vmType string, val uint64)
	RecordVmForcedPreemptionCount(vmType string, val uint64)
	RecordVmIdleStepCountThread0(vmType string, val uint64)
}

// TypedVmMetricer matches VmMetricer except the vmType parameter is already baked in and not supplied to each method
type TypedVmMetricer interface {
	RecordExecutionTime(t time.Duration)
	RecordMemoryUsed(memoryUsed uint64)
	RecordSteps(val uint64)
	RecordRmwSuccessCount(val uint64)
	RecordRmwFailCount(val uint64)
	RecordMaxStepsBetweenLLAndSC(val uint64)
	RecordReservationInvalidationCount(val uint64)
	RecordForcedPreemptionCount(val uint64)
	RecordIdleStepCountThread0(val uint64)
}

type VmMetrics struct {
	vmExecutionTime            *prometheus.HistogramVec
	vmMemoryUsed               *prometheus.HistogramVec
	vmSteps                    *prometheus.GaugeVec
	vmRmwSuccessCount          *prometheus.GaugeVec
	vmRmwFailCount             *prometheus.GaugeVec
	vmMaxStepsBetweenLLAndSC   *prometheus.GaugeVec
	vmReservationInvalidations *prometheus.GaugeVec
	vmForcedPreemptions        *prometheus.GaugeVec
	vmIdleStepsThread0         *prometheus.GaugeVec
}

var _ VmMetricer = (*VmMetrics)(nil)

func (m *VmMetrics) RecordVmExecutionTime(vmType string, dur time.Duration) {
	m.vmExecutionTime.WithLabelValues(vmType).Observe(dur.Seconds())
}

func (m *VmMetrics) RecordVmMemoryUsed(vmType string, memoryUsed uint64) {
	m.vmMemoryUsed.WithLabelValues(vmType).Observe(float64(memoryUsed))
}

func (m *VmMetrics) RecordVmSteps(vmType string, val uint64) {
	m.vmSteps.WithLabelValues(vmType).Set(float64(val))
}

func (m *VmMetrics) RecordVmRmwSuccessCount(vmType string, val uint64) {
	m.vmRmwSuccessCount.WithLabelValues(vmType).Set(float64(val))
}

func (m *VmMetrics) RecordVmRmwFailCount(vmType string, val uint64) {
	m.vmRmwFailCount.WithLabelValues(vmType).Set(float64(val))
}

func (m *VmMetrics) RecordVmMaxStepsBetweenLLAndSC(vmType string, val uint64) {
	m.vmMaxStepsBetweenLLAndSC.WithLabelValues(vmType).Set(float64(val))
}

func (m *VmMetrics) RecordVmReservationInvalidationCount(vmType string, val uint64) {
	m.vmReservationInvalidations.WithLabelValues(vmType).Set(float64(val))
}

func (m *VmMetrics) RecordVmForcedPreemptionCount(vmType string, val uint64) {
	m.vmForcedPreemptions.WithLabelValues(vmType).Set(float64(val))
}

func (m *VmMetrics) RecordVmIdleStepCountThread0(vmType string, val uint64) {
	m.vmIdleStepsThread0.WithLabelValues(vmType).Set(float64(val))
}

func NewVmMetrics(namespace string, factory metrics.Factory) *VmMetrics {
	return &VmMetrics{
		vmExecutionTime: factory.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "vm_execution_time",
			Help:      "Time (in seconds) to execute the fault proof VM",
			Buckets: append(
				[]float64{1.0, 10.0},
				prometheus.ExponentialBuckets(30.0, 2.0, 14)...),
		}, []string{"vm"}),
		vmMemoryUsed: factory.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "vm_memory_used",
			Help:      "Memory used (in bytes) to execute the fault proof VM",
			// 100MiB increments from 0 to 1.5GiB
			Buckets: prometheus.LinearBuckets(0, 1024*1024*100, 15),
		}, []string{"vm"}),
		vmSteps: factory.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "vm_step_count",
			Help:      "Number of steps executed during vm run",
		}, []string{"vm"}),
		vmRmwSuccessCount: factory.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "vm_rmw_success_count",
			Help:      "Number of successful RMW instruction sequences during vm run",
		}, []string{"vm"}),
		vmRmwFailCount: factory.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "vm_rmw_fail_count",
			Help:      "Number of failed RMW instruction sequences during vm run",
		}, []string{"vm"}),
		vmMaxStepsBetweenLLAndSC: factory.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "vm_max_steps_between_ll_and_sc",
			Help:      "The maximum number of steps observed between matching ll(d) and sc(d) instructions during the vm run",
		}, []string{"vm"}),
		vmReservationInvalidations: factory.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "vm_reservation_invalidations",
			Help:      "Number of memory reservations that were invalidated during vm run",
		}, []string{"vm"}),
		vmForcedPreemptions: factory.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "vm_forced_preemptions",
			Help:      "Number of forced preemptions during vm run",
		}, []string{"vm"}),
		vmIdleStepsThread0: factory.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "vm_idle_steps_thread0",
			Help:      "Number of steps thread 0 is idle during vm run",
		}, []string{"vm"}),
	}
}

type NoopVmMetrics struct{}

var _ VmMetricer = NoopVmMetrics{}

func (n NoopVmMetrics) RecordVmExecutionTime(vmType string, t time.Duration)           {}
func (n NoopVmMetrics) RecordVmMemoryUsed(vmType string, memoryUsed uint64)            {}
func (n NoopVmMetrics) RecordVmSteps(vmType string, val uint64)                        {}
func (n NoopVmMetrics) RecordVmRmwSuccessCount(vmType string, val uint64)              {}
func (n NoopVmMetrics) RecordVmRmwFailCount(vmType string, val uint64)                 {}
func (n NoopVmMetrics) RecordVmMaxStepsBetweenLLAndSC(vmType string, val uint64)       {}
func (n NoopVmMetrics) RecordVmReservationInvalidationCount(vmType string, val uint64) {}
func (n NoopVmMetrics) RecordVmForcedPreemptionCount(vmType string, val uint64)        {}
func (n NoopVmMetrics) RecordVmIdleStepCountThread0(vmType string, val uint64)         {}

type typedVmMetricsImpl struct {
	m      VmMetricer
	vmType string
}

var _ TypedVmMetricer = (*typedVmMetricsImpl)(nil)

func (m *typedVmMetricsImpl) RecordExecutionTime(dur time.Duration) {
	m.m.RecordVmExecutionTime(m.vmType, dur)
}

func (m *typedVmMetricsImpl) RecordMemoryUsed(memoryUsed uint64) {
	m.m.RecordVmMemoryUsed(m.vmType, memoryUsed)
}

func (m *typedVmMetricsImpl) RecordSteps(val uint64) {
	m.m.RecordVmSteps(m.vmType, val)
}

func (m *typedVmMetricsImpl) RecordRmwSuccessCount(val uint64) {
	m.m.RecordVmRmwSuccessCount(m.vmType, val)
}

func (m *typedVmMetricsImpl) RecordRmwFailCount(val uint64) {
	m.m.RecordVmRmwFailCount(m.vmType, val)
}

func (m *typedVmMetricsImpl) RecordMaxStepsBetweenLLAndSC(val uint64) {
	m.m.RecordVmMaxStepsBetweenLLAndSC(m.vmType, val)
}

func (m *typedVmMetricsImpl) RecordReservationInvalidationCount(val uint64) {
	m.m.RecordVmReservationInvalidationCount(m.vmType, val)
}

func (m *typedVmMetricsImpl) RecordForcedPreemptionCount(val uint64) {
	m.m.RecordVmForcedPreemptionCount(m.vmType, val)
}

func (m *typedVmMetricsImpl) RecordIdleStepCountThread0(val uint64) {
	m.m.RecordVmIdleStepCountThread0(m.vmType, val)
}

func NewTypedVmMetrics(m VmMetricer, vmType string) TypedVmMetricer {
	return &typedVmMetricsImpl{
		m:      m,
		vmType: vmType,
	}
}
