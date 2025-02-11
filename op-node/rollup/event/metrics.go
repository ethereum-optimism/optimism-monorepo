package event

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/ethereum-optimism/optimism/op-service/metrics"
)

type Metrics interface {
	RecordEmittedEvent(eventName string, emitter string)
	RecordProcessedEvent(eventName string, deriver string, duration time.Duration)
	RecordEventsRateLimited()
}

type NoopMetrics struct {
}

func (n NoopMetrics) RecordEmittedEvent(eventName string, emitter string) {}

func (n NoopMetrics) RecordProcessedEvent(eventName string, deriver string, duration time.Duration) {}

func (n NoopMetrics) RecordEventsRateLimited() {}

var _ Metrics = NoopMetrics{}

type EventMetricsTracker struct {
	EmittedEvents   *prometheus.CounterVec
	ProcessedEvents *prometheus.CounterVec

	// We don't use a histogram for observing time durations,
	// as each vec entry (event-type, deriver type) is synchronous with other occurrences of the same entry key,
	// so we can get a reasonably good understanding of execution by looking at the rate.
	// Bucketing to detect outliers would be nice, but also increases the overhead by a lot,
	// where we already track many event-type/deriver combinations.
	EventsProcessTime *prometheus.CounterVec

	EventsRateLimited *metrics.Event
}

func NewMetricsTracker(ns string, factory metrics.Factory) *EventMetricsTracker {
	return &EventMetricsTracker{
		EmittedEvents: factory.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: ns,
				Subsystem: "events",
				Name:      "emitted",
				Help:      "number of emitted events",
			}, []string{"event_type", "emitter"}),

		ProcessedEvents: factory.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: ns,
				Subsystem: "events",
				Name:      "processed",
				Help:      "number of processed events",
			}, []string{"event_type", "deriver"}),

		EventsProcessTime: factory.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: ns,
				Subsystem: "events",
				Name:      "process_time",
				Help:      "total duration in seconds of processed events",
			}, []string{"event_type", "deriver"}),

		EventsRateLimited: metrics.NewEvent(factory, ns, "events", "rate_limited", "events rate limiter hits"),
	}
}

func (m *EventMetricsTracker) RecordEmittedEvent(eventName string, emitter string) {
	m.EmittedEvents.WithLabelValues(eventName, emitter).Inc()
}

func (m *EventMetricsTracker) RecordProcessedEvent(eventName string, deriver string, duration time.Duration) {
	m.ProcessedEvents.WithLabelValues(eventName, deriver).Inc()
	// We take the absolute value; if the clock was not monotonically increased between start and top,
	// there still was a duration gap. And the Counter metrics-type would panic if the duration is negative.
	m.EventsProcessTime.WithLabelValues(eventName, deriver).Add(float64(duration.Abs()) / float64(time.Second))
}

func (m *EventMetricsTracker) RecordEventsRateLimited() {
	m.EventsRateLimited.Record()
}
