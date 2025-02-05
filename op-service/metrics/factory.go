package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Factory interface {
	NewCounter(opts prometheus.CounterOpts) prometheus.Counter
	NewCounterVec(opts prometheus.CounterOpts, labelNames []string) *prometheus.CounterVec
	NewGauge(opts prometheus.GaugeOpts) prometheus.Gauge
	NewGaugeFunc(opts prometheus.GaugeOpts, function func() float64) prometheus.GaugeFunc
	NewGaugeVec(opts prometheus.GaugeOpts, labelNames []string) *prometheus.GaugeVec
	NewHistogram(opts prometheus.HistogramOpts) prometheus.Histogram
	NewHistogramVec(opts prometheus.HistogramOpts, labelNames []string) *prometheus.HistogramVec
	NewSummary(opts prometheus.SummaryOpts) prometheus.Summary
	NewSummaryVec(opts prometheus.SummaryOpts, labelNames []string) *prometheus.SummaryVec
	Document() []DocumentedMetric
}

type DocumentedMetric struct {
	Type   string   `json:"type"`
	Name   string   `json:"name"`
	Help   string   `json:"help"`
	Labels []string `json:"labels"`
}

type documentor struct {
	metrics []DocumentedMetric
	factory promauto.Factory
}

func With(registry *prometheus.Registry) Factory {
	return &documentor{
		factory: promauto.With(registry),
	}
}

func (d *documentor) NewCounter(opts prometheus.CounterOpts) prometheus.Counter {
	d.metrics = append(d.metrics, DocumentedMetric{
		Type: "counter",
		Name: fullName(opts.Namespace, opts.Subsystem, opts.Name),
		Help: opts.Help,
	})
	return d.factory.NewCounter(opts)
}

func (d *documentor) NewCounterVec(opts prometheus.CounterOpts, labelNames []string) *prometheus.CounterVec {
	d.metrics = append(d.metrics, DocumentedMetric{
		Type:   "counter",
		Name:   fullName(opts.Namespace, opts.Subsystem, opts.Name),
		Help:   opts.Help,
		Labels: labelNames,
	})
	return d.factory.NewCounterVec(opts, labelNames)
}

func (d *documentor) NewGauge(opts prometheus.GaugeOpts) prometheus.Gauge {
	d.metrics = append(d.metrics, DocumentedMetric{
		Type: "gauge",
		Name: fullName(opts.Namespace, opts.Subsystem, opts.Name),
		Help: opts.Help,
	})
	return d.factory.NewGauge(opts)
}

func (d *documentor) NewGaugeFunc(opts prometheus.GaugeOpts, function func() float64) prometheus.GaugeFunc {
	d.metrics = append(d.metrics, DocumentedMetric{
		Type: "gauge",
		Name: fullName(opts.Namespace, opts.Subsystem, opts.Name),
		Help: opts.Help,
	})
	return d.factory.NewGaugeFunc(opts, function)
}

func (d *documentor) NewGaugeVec(opts prometheus.GaugeOpts, labelNames []string) *prometheus.GaugeVec {
	d.metrics = append(d.metrics, DocumentedMetric{
		Type:   "gauge",
		Name:   fullName(opts.Namespace, opts.Subsystem, opts.Name),
		Help:   opts.Help,
		Labels: labelNames,
	})
	return d.factory.NewGaugeVec(opts, labelNames)
}

func (d *documentor) NewHistogram(opts prometheus.HistogramOpts) prometheus.Histogram {
	d.metrics = append(d.metrics, DocumentedMetric{
		Type: "histogram",
		Name: fullName(opts.Namespace, opts.Subsystem, opts.Name),
		Help: opts.Help,
	})
	return d.factory.NewHistogram(opts)
}

func (d *documentor) NewHistogramVec(opts prometheus.HistogramOpts, labelNames []string) *prometheus.HistogramVec {
	d.metrics = append(d.metrics, DocumentedMetric{
		Type:   "histogram",
		Name:   fullName(opts.Namespace, opts.Subsystem, opts.Name),
		Help:   opts.Help,
		Labels: labelNames,
	})
	return d.factory.NewHistogramVec(opts, labelNames)
}

func (d *documentor) NewSummary(opts prometheus.SummaryOpts) prometheus.Summary {
	d.metrics = append(d.metrics, DocumentedMetric{
		Type: "summary",
		Name: fullName(opts.Namespace, opts.Subsystem, opts.Name),
		Help: opts.Help,
	})
	return d.factory.NewSummary(opts)
}

func (d *documentor) NewSummaryVec(opts prometheus.SummaryOpts, labelNames []string) *prometheus.SummaryVec {
	d.metrics = append(d.metrics, DocumentedMetric{
		Type:   "summary",
		Name:   fullName(opts.Namespace, opts.Subsystem, opts.Name),
		Help:   opts.Help,
		Labels: labelNames,
	})
	return d.factory.NewSummaryVec(opts, labelNames)
}

func (d *documentor) Document() []DocumentedMetric {
	return d.metrics
}

func fullName(ns, subsystem, name string) string {
	out := ns
	if subsystem != "" {
		out += "_" + subsystem
	}
	return out + "_" + name
}
