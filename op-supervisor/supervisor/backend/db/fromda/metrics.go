package fromda

type Metrics interface {
	RecordDBDerivedEntryCount(count int64)
}

type ChainMetrics interface {
	RecordDBEntryCount(kind string, count int64)
}

type delegate struct {
	inner ChainMetrics
	kind  string
}

func (d *delegate) RecordDBDerivedEntryCount(count int64) {
	d.inner.RecordDBEntryCount(d.kind, count)
}

func AdaptMetrics(chainMetrics ChainMetrics, kind string) Metrics {
	return &delegate{
		kind:  kind,
		inner: chainMetrics,
	}
}
