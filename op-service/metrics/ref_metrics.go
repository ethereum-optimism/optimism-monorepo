package metrics

import (
	"encoding/binary"
	"time"

	"github.com/ethereum-optimism/optimism/op-service/eth"

	"github.com/ethereum/go-ethereum/common"
	"github.com/prometheus/client_golang/prometheus"
)

type RefMetricer interface {
	RecordRef(layer string, name string, num uint64, timestamp uint64, h common.Hash)
	RecordL1Ref(name string, ref eth.L1BlockRef)
	RecordL2Ref(name string, ref eth.L2BlockRef)
}

// RefMetrics provides block reference metrics. It's a metrics module that's
// supposed to be embedded into a service metrics type. The service metrics type
// should set the full namespace and create the factory before calling
// NewRefMetrics.
type RefMetrics struct {
	RefsNumber  *prometheus.GaugeVec
	RefsTime    *prometheus.GaugeVec
	RefsHash    *prometheus.GaugeVec
	RefsSeqNr   *prometheus.GaugeVec
	RefsLatency *prometheus.GaugeVec
	// hash of the last seen block per name, so we don't reduce/increase latency on updates of the same data,
	// and only count the first occurrence
	LatencySeen map[string]common.Hash
}

var _ RefMetricer = (*RefMetrics)(nil)

// MakeRefMetrics returns a new RefMetrics, initializing its prometheus fields
// using factory. It is supposed to be used inside the constructors of metrics
// structs for any op service after the full namespace and factory have been
// setup.
//
// ns is the fully qualified namespace, e.g. "op_node_default".
func MakeRefMetrics(ns string, factory Factory) RefMetrics {
	return RefMetrics{
		RefsNumber: factory.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns,
			Name:      "refs_number",
			Help:      "Gauge representing the different L1/L2 reference block numbers",
		}, []string{
			"layer",
			"type",
		}),
		RefsTime: factory.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns,
			Name:      "refs_time",
			Help:      "Gauge representing the different L1/L2 reference block timestamps",
		}, []string{
			"layer",
			"type",
		}),
		RefsHash: factory.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns,
			Name:      "refs_hash",
			Help:      "Gauge representing the different L1/L2 reference block hashes truncated to float values",
		}, []string{
			"layer",
			"type",
		}),
		RefsSeqNr: factory.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns,
			Name:      "refs_seqnr",
			Help:      "Gauge representing the different L2 reference sequence numbers",
		}, []string{
			"type",
		}),
		RefsLatency: factory.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns,
			Name:      "refs_latency",
			Help:      "Gauge representing the different L1/L2 reference block timestamps minus current time, in seconds",
		}, []string{
			"layer",
			"type",
		}),
		LatencySeen: make(map[string]common.Hash),
	}
}

func (m *RefMetrics) RecordRef(layer string, name string, num uint64, timestamp uint64, h common.Hash) {
	m.RefsNumber.WithLabelValues(layer, name).Set(float64(num))
	if timestamp != 0 {
		m.RefsTime.WithLabelValues(layer, name).Set(float64(timestamp))
		// only meter the latency when we first see this hash for the given label name
		if m.LatencySeen[name] != h {
			m.LatencySeen[name] = h
			m.RefsLatency.WithLabelValues(layer, name).Set(float64(timestamp) - (float64(time.Now().UnixNano()) / 1e9))
		}
	}
	// we map the first 8 bytes to a float64, so we can graph changes of the hash to find divergences visually.
	// We don't do math.Float64frombits, just a regular conversion, to keep the value within a manageable range.
	m.RefsHash.WithLabelValues(layer, name).Set(float64(binary.LittleEndian.Uint64(h[:])))
}

func (m *RefMetrics) RecordL1Ref(name string, ref eth.L1BlockRef) {
	m.RecordRef("l1", name, ref.Number, ref.Time, ref.Hash)
}

func (m *RefMetrics) RecordL2Ref(name string, ref eth.L2BlockRef) {
	m.RecordRef("l2", name, ref.Number, ref.Time, ref.Hash)
	m.RecordRef("l1_origin", name, ref.L1Origin.Number, 0, ref.L1Origin.Hash)
	m.RefsSeqNr.WithLabelValues(name).Set(float64(ref.SequenceNumber))
}

// NoopRefMetrics can be embedded in a noop version of a metric implementation
// to have a noop RefMetricer.
type NoopRefMetrics struct{}

func (*NoopRefMetrics) RecordRef(string, string, uint64, uint64, common.Hash) {}
func (*NoopRefMetrics) RecordL1Ref(string, eth.L1BlockRef)                    {}
func (*NoopRefMetrics) RecordL2Ref(string, eth.L2BlockRef)                    {}
