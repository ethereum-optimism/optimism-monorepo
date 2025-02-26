package metrics

type NoopMetrics struct{}

func (n NoopMetrics) RecordInfo(version string) {}

func (n NoopMetrics) RecordUp() {}

var _ Metricer = NoopMetrics{}
