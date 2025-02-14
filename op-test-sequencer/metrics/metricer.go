package metrics

type Metricer interface {
	RecordInfo(version string)
	RecordUp()
}
