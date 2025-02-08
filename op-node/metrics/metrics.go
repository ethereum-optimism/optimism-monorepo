// Package metrics provides a set of metrics for the op-node.
package metrics

import (
	"context"
	"net"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/params"

	altda "github.com/ethereum-optimism/optimism/op-alt-da"
	"github.com/ethereum-optimism/optimism/op-node/p2p/store"
	"github.com/ethereum-optimism/optimism/op-node/rollup/event"
	ophttp "github.com/ethereum-optimism/optimism/op-service/httputil"
	"github.com/ethereum-optimism/optimism/op-service/metrics"

	pb "github.com/libp2p/go-libp2p-pubsub/pb"
	libp2pmetrics "github.com/libp2p/go-libp2p/core/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/ethereum/go-ethereum/common"

	"github.com/ethereum-optimism/optimism/op-service/eth"
)

const Namespace = "op_node"

type Metricer interface {
	RecordInfo(version string)
	RecordUp()
	RecordRPCServerRequest(method string) func()
	RecordRPCClientRequest(method string) func(err error)
	RecordRPCClientResponse(method string, err error)
	SetDerivationIdle(status bool)
	SetSequencerState(active bool)
	RecordPipelineReset()
	RecordSequencingError()
	RecordPublishingError()
	RecordDerivationError()
	RecordEmittedEvent(eventName string, emitter string)
	RecordProcessedEvent(eventName string, deriver string, duration time.Duration)
	RecordEventsRateLimited()
	RecordReceivedUnsafePayload(payload *eth.ExecutionPayloadEnvelope)
	RecordRef(layer string, name string, num uint64, timestamp uint64, h common.Hash)
	RecordL1Ref(name string, ref eth.L1BlockRef)
	RecordL2Ref(name string, ref eth.L2BlockRef)
	RecordUnsafePayloadsBuffer(length uint64, memSize uint64, next eth.BlockID)
	RecordDerivedBatches(batchType string)
	CountSequencedTxsInBlock(txns int, deposits int)
	RecordL1ReorgDepth(d uint64)
	RecordSequencerInconsistentL1Origin(from eth.BlockID, to eth.BlockID)
	RecordSequencerReset()
	RecordGossipEvent(evType int32)
	IncPeerCount()
	DecPeerCount()
	IncStreamCount()
	DecStreamCount()
	RecordBandwidth(ctx context.Context, bwc *libp2pmetrics.BandwidthCounter)
	RecordSequencerBuildingDiffTime(duration time.Duration)
	RecordSequencerSealingTime(duration time.Duration)
	Document() []metrics.DocumentedMetric
	RecordChannelInputBytes(num int)
	RecordHeadChannelOpened()
	RecordChannelTimedOut()
	RecordFrame()
	// P2P Metrics
	SetPeerScores(allScores []store.PeerScores)
	ClientPayloadByNumberEvent(num uint64, resultCode byte, duration time.Duration)
	ServerPayloadByNumberEvent(num uint64, resultCode byte, duration time.Duration)
	PayloadsQuarantineSize(n int)
	RecordPeerUnban()
	RecordIPUnban()
	RecordDial(allow bool)
	RecordAccept(allow bool)
	ReportProtocolVersions(local, engine, recommended, required params.ProtocolVersion)
}

// Metrics tracks all the metrics for the op-node.
type Metrics struct {
	Info *prometheus.GaugeVec
	Up   prometheus.Gauge

	metrics.RPCMetrics

	L1SourceCache *metrics.CacheMetrics
	L2SourceCache *metrics.CacheMetrics

	DerivationIdle prometheus.Gauge

	PipelineResets   *metrics.Event
	UnsafePayloads   *metrics.Event
	DerivationErrors *metrics.Event
	SequencingErrors *metrics.Event
	PublishingErrors *metrics.Event
	SequencerActive  prometheus.Gauge

	*event.EventMetricsTracker

	DerivedBatches metrics.EventVec

	P2PReqDurationSeconds *prometheus.HistogramVec
	P2PReqTotal           *prometheus.CounterVec
	P2PPayloadByNumber    *prometheus.GaugeVec

	PayloadsQuarantineTotal prometheus.Gauge

	SequencerInconsistentL1Origin *metrics.Event
	SequencerResets               *metrics.Event

	L1RequestDurationSeconds *prometheus.HistogramVec

	SequencerBuildingDiffDurationSeconds prometheus.Histogram
	SequencerBuildingDiffTotal           prometheus.Counter

	SequencerSealingDurationSeconds prometheus.Histogram
	SequencerSealingTotal           prometheus.Counter

	UnsafePayloadsBufferLen     prometheus.Gauge
	UnsafePayloadsBufferMemSize prometheus.Gauge

	metrics.RefMetrics

	L1ReorgDepth prometheus.Histogram

	TransactionsSequencedTotal *prometheus.CounterVec

	AltDAMetrics altda.Metricer

	// Channel Bank Metrics
	headChannelOpenedEvent *metrics.Event
	channelTimedOutEvent   *metrics.Event
	frameAddedEvent        *metrics.Event

	// P2P Metrics
	PeerCount         prometheus.Gauge
	StreamCount       prometheus.Gauge
	GossipEventsTotal *prometheus.CounterVec
	BandwidthTotal    *prometheus.GaugeVec
	PeerUnbans        prometheus.Counter
	IPUnbans          prometheus.Counter
	Dials             *prometheus.CounterVec
	Accepts           *prometheus.CounterVec
	PeerScores        *prometheus.HistogramVec

	ChannelInputBytes prometheus.Counter

	// Protocol version reporting
	// Delta = params.ProtocolVersionComparison
	ProtocolVersionDelta *prometheus.GaugeVec
	// ProtocolVersions is pseudo-metric to report the exact protocol version info
	ProtocolVersions *prometheus.GaugeVec

	registry *prometheus.Registry
	factory  metrics.Factory
}

var _ Metricer = (*Metrics)(nil)

// NewMetrics creates a new [Metrics] instance with the given process name.
func NewMetrics(procName string) *Metrics {
	if procName == "" {
		procName = "default"
	}
	ns := Namespace + "_" + procName

	registry := prometheus.NewRegistry()
	registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	registry.MustRegister(collectors.NewGoCollector())
	factory := metrics.With(registry)

	return &Metrics{
		Info: factory.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns,
			Name:      "info",
			Help:      "Pseudo-metric tracking version and config info",
		}, []string{
			"version",
		}),
		Up: factory.NewGauge(prometheus.GaugeOpts{
			Namespace: ns,
			Name:      "up",
			Help:      "1 if the op node has finished starting up",
		}),

		RPCMetrics: metrics.MakeRPCMetrics(ns, factory),

		L1SourceCache: metrics.NewCacheMetrics(factory, ns, "l1_source_cache", "L1 Source cache"),
		L2SourceCache: metrics.NewCacheMetrics(factory, ns, "l2_source_cache", "L2 Source cache"),

		DerivationIdle: factory.NewGauge(prometheus.GaugeOpts{
			Namespace: ns,
			Name:      "derivation_idle",
			Help:      "1 if the derivation pipeline is idle",
		}),

		PipelineResets:   metrics.NewEvent(factory, ns, "", "pipeline_resets", "derivation pipeline resets"),
		UnsafePayloads:   metrics.NewEvent(factory, ns, "", "unsafe_payloads", "unsafe payloads"),
		DerivationErrors: metrics.NewEvent(factory, ns, "", "derivation_errors", "derivation errors"),
		SequencingErrors: metrics.NewEvent(factory, ns, "", "sequencing_errors", "sequencing errors"),
		PublishingErrors: metrics.NewEvent(factory, ns, "", "publishing_errors", "p2p publishing errors"),
		SequencerActive: factory.NewGauge(prometheus.GaugeOpts{
			Namespace: ns,
			Name:      "sequencer_active",
			Help:      "1 if sequencer active, 0 otherwise",
		}),
		EventMetricsTracker: event.NewMetricsTracker(ns, factory),

		DerivedBatches: metrics.NewEventVec(factory, ns, "", "derived_batches", "derived batches", []string{"type"}),

		SequencerInconsistentL1Origin: metrics.NewEvent(factory, ns, "", "sequencer_inconsistent_l1_origin", "events when the sequencer selects an inconsistent L1 origin"),
		SequencerResets:               metrics.NewEvent(factory, ns, "", "sequencer_resets", "sequencer resets"),

		UnsafePayloadsBufferLen: factory.NewGauge(prometheus.GaugeOpts{
			Namespace: ns,
			Name:      "unsafe_payloads_buffer_len",
			Help:      "Number of buffered L2 unsafe payloads",
		}),
		UnsafePayloadsBufferMemSize: factory.NewGauge(prometheus.GaugeOpts{
			Namespace: ns,
			Name:      "unsafe_payloads_buffer_mem_size",
			Help:      "Total estimated memory size of buffered L2 unsafe payloads",
		}),

		RefMetrics: metrics.MakeRefMetrics(ns, factory),

		L1ReorgDepth: factory.NewHistogram(prometheus.HistogramOpts{
			Namespace: ns,
			Name:      "l1_reorg_depth",
			Buckets:   []float64{0.5, 1.5, 2.5, 3.5, 4.5, 5.5, 6.5, 7.5, 8.5, 9.5, 10.5, 20.5, 50.5, 100.5},
			Help:      "Histogram of L1 Reorg Depths",
		}),

		TransactionsSequencedTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: ns,
			Name:      "transactions_sequenced_total",
			Help:      "Count of total transactions sequenced",
		}, []string{"type"}),
		PeerCount: factory.NewGauge(prometheus.GaugeOpts{
			Namespace: ns,
			Subsystem: "p2p",
			Name:      "peer_count",
			Help:      "Count of currently connected p2p peers",
		}),
		PeerScores: factory.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: ns,
			Name:      "peer_scores",
			Help:      "Histogram of currently connected peer scores",
			Buckets:   []float64{-100, -40, -20, -10, -5, -2, -1, -0.5, -0.05, 0, 0.05, 0.5, 1, 2, 5, 10, 20, 40},
		}, []string{"type"}),
		StreamCount: factory.NewGauge(prometheus.GaugeOpts{
			Namespace: ns,
			Subsystem: "p2p",
			Name:      "stream_count",
			Help:      "Count of currently connected p2p streams",
		}),
		GossipEventsTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: ns,
			Subsystem: "p2p",
			Name:      "gossip_events_total",
			Help:      "Count of gossip events by type",
		}, []string{
			"type",
		}),
		BandwidthTotal: factory.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns,
			Subsystem: "p2p",
			Name:      "bandwidth_bytes_total",
			Help:      "P2P bandwidth by direction",
		}, []string{
			"direction",
		}),
		PeerUnbans: factory.NewCounter(prometheus.CounterOpts{
			Namespace: ns,
			Subsystem: "p2p",
			Name:      "peer_unbans",
			Help:      "Count of peer unbans",
		}),
		IPUnbans: factory.NewCounter(prometheus.CounterOpts{
			Namespace: ns,
			Subsystem: "p2p",
			Name:      "ip_unbans",
			Help:      "Count of IP unbans",
		}),
		Dials: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: ns,
			Subsystem: "p2p",
			Name:      "dials",
			Help:      "Count of outgoing dial attempts, with label to filter to allowed attempts",
		}, []string{"allow"}),
		Accepts: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: ns,
			Subsystem: "p2p",
			Name:      "accepts",
			Help:      "Count of incoming dial attempts to accept, with label to filter to allowed attempts",
		}, []string{"allow"}),

		headChannelOpenedEvent: metrics.NewEvent(factory, ns, "", "head_channel", "New channel at the front of the channel bank"),
		channelTimedOutEvent:   metrics.NewEvent(factory, ns, "", "channel_timeout", "Channel has timed out"),
		frameAddedEvent:        metrics.NewEvent(factory, ns, "", "frame_added", "New frame ingested in the channel bank"),

		ChannelInputBytes: factory.NewCounter(prometheus.CounterOpts{
			Namespace: ns,
			Name:      "channel_input_bytes",
			Help:      "Number of compressed bytes added to the channel",
		}),

		P2PReqDurationSeconds: factory.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: ns,
			Subsystem: "p2p",
			Name:      "req_duration_seconds",
			Buckets:   []float64{},
			Help:      "Duration of P2P requests",
		}, []string{
			"p2p_role", // "client" or "server"
			"p2p_method",
			"result_code",
		}),

		P2PReqTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: ns,
			Subsystem: "p2p",
			Name:      "req_total",
			Help:      "Number of P2P requests",
		}, []string{
			"p2p_role", // "client" or "server"
			"p2p_method",
			"result_code",
		}),

		P2PPayloadByNumber: factory.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns,
			Subsystem: "p2p",
			Name:      "payload_by_number",
			Help:      "Payload by number requests",
		}, []string{
			"p2p_role", // "client" or "server"
		}),
		PayloadsQuarantineTotal: factory.NewGauge(prometheus.GaugeOpts{
			Namespace: ns,
			Subsystem: "p2p",
			Name:      "payloads_quarantine_total",
			Help:      "number of unverified execution payloads buffered in quarantine",
		}),

		L1RequestDurationSeconds: factory.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: ns,
			Name:      "l1_request_seconds",
			Buckets: []float64{
				.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
			Help: "Histogram of L1 request time",
		}, []string{"request"}),

		SequencerBuildingDiffDurationSeconds: factory.NewHistogram(prometheus.HistogramOpts{
			Namespace: ns,
			Name:      "sequencer_building_diff_seconds",
			Buckets: []float64{
				-10, -5, -2.5, -1, -.5, -.25, -.1, -0.05, -0.025, -0.01, -0.005,
				.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
			Help: "Histogram of Sequencer building time, minus block time",
		}),
		SequencerBuildingDiffTotal: factory.NewCounter(prometheus.CounterOpts{
			Namespace: ns,
			Name:      "sequencer_building_diff_total",
			Help:      "Number of sequencer block building jobs",
		}),
		SequencerSealingDurationSeconds: factory.NewHistogram(prometheus.HistogramOpts{
			Namespace: ns,
			Name:      "sequencer_sealing_seconds",
			Buckets:   []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
			Help:      "Histogram of Sequencer block sealing time",
		}),
		SequencerSealingTotal: factory.NewCounter(prometheus.CounterOpts{
			Namespace: ns,
			Name:      "sequencer_sealing_total",
			Help:      "Number of sequencer block sealing jobs",
		}),

		ProtocolVersionDelta: factory.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns,
			Name:      "protocol_version_delta",
			Help:      "Difference between local and global protocol version, and execution-engine, per type of version",
		}, []string{
			"type",
		}),
		ProtocolVersions: factory.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns,
			Name:      "protocol_versions",
			Help:      "Pseudo-metric tracking recommended and required protocol version info",
		}, []string{
			"local",
			"engine",
			"recommended",
			"required",
		}),

		AltDAMetrics: altda.MakeMetrics(ns, factory),

		registry: registry,
		factory:  factory,
	}
}

// SetPeerScores updates the peer score metrics.
// Accepts a slice of peer scores in any order.
func (m *Metrics) SetPeerScores(allScores []store.PeerScores) {
	for _, scores := range allScores {
		m.PeerScores.WithLabelValues("total").Observe(scores.Gossip.Total)
		m.PeerScores.WithLabelValues("ipColocation").Observe(scores.Gossip.IPColocationFactor)
		m.PeerScores.WithLabelValues("behavioralPenalty").Observe(scores.Gossip.BehavioralPenalty)
		m.PeerScores.WithLabelValues("blocksFirstMessage").Observe(scores.Gossip.Blocks.FirstMessageDeliveries)
		m.PeerScores.WithLabelValues("blocksTimeInMesh").Observe(scores.Gossip.Blocks.TimeInMesh)
		m.PeerScores.WithLabelValues("blocksMessageDeliveries").Observe(scores.Gossip.Blocks.MeshMessageDeliveries)
		m.PeerScores.WithLabelValues("blocksInvalidMessageDeliveries").Observe(scores.Gossip.Blocks.InvalidMessageDeliveries)

		m.PeerScores.WithLabelValues("reqRespValidResponses").Observe(scores.ReqResp.ValidResponses)
		m.PeerScores.WithLabelValues("reqRespErrorResponses").Observe(scores.ReqResp.ErrorResponses)
		m.PeerScores.WithLabelValues("reqRespRejectedPayloads").Observe(scores.ReqResp.RejectedPayloads)
	}
}

// RecordInfo sets a pseudo-metric that contains versioning and
// config info for the opnode.
func (m *Metrics) RecordInfo(version string) {
	m.Info.WithLabelValues(version).Set(1)
}

// RecordUp sets the up metric to 1.
func (m *Metrics) RecordUp() {
	m.Up.Set(1)
}

func (m *Metrics) SetDerivationIdle(status bool) {
	var val float64
	if status {
		val = 1
	}
	m.DerivationIdle.Set(val)
}

func (m *Metrics) SetSequencerState(active bool) {
	var val float64
	if active {
		val = 1
	}
	m.SequencerActive.Set(val)
}

func (m *Metrics) RecordPipelineReset() {
	m.PipelineResets.Record()
}

func (m *Metrics) RecordSequencingError() {
	m.SequencingErrors.Record()
}

func (m *Metrics) RecordPublishingError() {
	m.PublishingErrors.Record()
}

func (m *Metrics) RecordDerivationError() {
	m.DerivationErrors.Record()
}

func (m *Metrics) RecordReceivedUnsafePayload(payload *eth.ExecutionPayloadEnvelope) {
	m.UnsafePayloads.Record()
	m.RecordRef("l2", "received_payload", uint64(payload.ExecutionPayload.BlockNumber), uint64(payload.ExecutionPayload.Timestamp), payload.ExecutionPayload.BlockHash)
}

func (m *Metrics) RecordUnsafePayloadsBuffer(length uint64, memSize uint64, next eth.BlockID) {
	m.RecordRef("l2", "l2_buffer_unsafe", next.Number, 0, next.Hash)
	m.UnsafePayloadsBufferLen.Set(float64(length))
	m.UnsafePayloadsBufferMemSize.Set(float64(memSize))
}

func (m *Metrics) RecordDerivedBatches(batchType string) {
	m.DerivedBatches.Record(batchType)
}

func (m *Metrics) CountSequencedTxsInBlock(txns int, deposits int) {
	m.TransactionsSequencedTotal.WithLabelValues("deposits").Add(float64(deposits))
	m.TransactionsSequencedTotal.WithLabelValues("txns").Add(float64(txns - deposits))
}

func (m *Metrics) RecordL1ReorgDepth(d uint64) {
	m.L1ReorgDepth.Observe(float64(d))
}

func (m *Metrics) RecordSequencerInconsistentL1Origin(from eth.BlockID, to eth.BlockID) {
	m.SequencerInconsistentL1Origin.Record()
	m.RecordRef("l1_origin", "inconsistent_from", from.Number, 0, from.Hash)
	m.RecordRef("l1_origin", "inconsistent_to", to.Number, 0, to.Hash)
}

func (m *Metrics) RecordSequencerReset() {
	m.SequencerResets.Record()
}

func (m *Metrics) RecordGossipEvent(evType int32) {
	m.GossipEventsTotal.WithLabelValues(pb.TraceEvent_Type_name[evType]).Inc()
}

func (m *Metrics) IncPeerCount() {
	m.PeerCount.Inc()
}

func (m *Metrics) DecPeerCount() {
	m.PeerCount.Dec()
}

func (m *Metrics) IncStreamCount() {
	m.StreamCount.Inc()
}

func (m *Metrics) DecStreamCount() {
	m.StreamCount.Dec()
}

func (m *Metrics) RecordBandwidth(ctx context.Context, bwc *libp2pmetrics.BandwidthCounter) {
	tick := time.NewTicker(10 * time.Second)
	defer tick.Stop()

	for {
		select {
		case <-tick.C:
			bwTotals := bwc.GetBandwidthTotals()
			m.BandwidthTotal.WithLabelValues("in").Set(float64(bwTotals.TotalIn))
			m.BandwidthTotal.WithLabelValues("out").Set(float64(bwTotals.TotalOut))
		case <-ctx.Done():
			return
		}
	}
}

// RecordL1RequestTime tracks the amount of time the derivation pipeline spent waiting for L1 data requests.
func (m *Metrics) RecordL1RequestTime(method string, duration time.Duration) {
	m.L1RequestDurationSeconds.WithLabelValues(method).Observe(float64(duration) / float64(time.Second))
}

// RecordSequencerBuildingDiffTime tracks the amount of time the sequencer was allowed between
// start to finish, incl. sealing, minus the block time.
// Ideally this is 0, realistically the sequencer scheduler may be busy with other jobs like syncing sometimes.
func (m *Metrics) RecordSequencerBuildingDiffTime(duration time.Duration) {
	m.SequencerBuildingDiffTotal.Inc()
	m.SequencerBuildingDiffDurationSeconds.Observe(float64(duration) / float64(time.Second))
}

// RecordSequencerSealingTime tracks the amount of time the sequencer took to finish sealing the block.
// Ideally this is 0, realistically it may take some time.
func (m *Metrics) RecordSequencerSealingTime(duration time.Duration) {
	m.SequencerSealingTotal.Inc()
	m.SequencerSealingDurationSeconds.Observe(float64(duration) / float64(time.Second))
}

// StartServer starts the metrics server on the given hostname and port.
func (m *Metrics) StartServer(hostname string, port int) (*ophttp.HTTPServer, error) {
	addr := net.JoinHostPort(hostname, strconv.Itoa(port))
	h := promhttp.InstrumentMetricHandler(
		m.registry, promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{}),
	)
	return ophttp.StartHTTPServer(addr, h)
}

func (m *Metrics) Document() []metrics.DocumentedMetric {
	return m.factory.Document()
}

func (m *Metrics) ClientPayloadByNumberEvent(num uint64, resultCode byte, duration time.Duration) {
	if resultCode > 4 { // summarize all high codes to reduce metrics overhead
		resultCode = 5
	}
	code := strconv.FormatUint(uint64(resultCode), 10)
	m.P2PReqTotal.WithLabelValues("client", "payload_by_number", code).Inc()
	m.P2PReqDurationSeconds.WithLabelValues("client", "payload_by_number", code).Observe(float64(duration) / float64(time.Second))
	m.P2PPayloadByNumber.WithLabelValues("client").Set(float64(num))
}

func (m *Metrics) ServerPayloadByNumberEvent(num uint64, resultCode byte, duration time.Duration) {
	code := strconv.FormatUint(uint64(resultCode), 10)
	m.P2PReqTotal.WithLabelValues("server", "payload_by_number", code).Inc()
	m.P2PReqDurationSeconds.WithLabelValues("server", "payload_by_number", code).Observe(float64(duration) / float64(time.Second))
	m.P2PPayloadByNumber.WithLabelValues("server").Set(float64(num))
}

func (m *Metrics) PayloadsQuarantineSize(n int) {
	m.PayloadsQuarantineTotal.Set(float64(n))
}

func (m *Metrics) RecordChannelInputBytes(inputCompressedBytes int) {
	m.ChannelInputBytes.Add(float64(inputCompressedBytes))
}

func (m *Metrics) RecordHeadChannelOpened() {
	m.headChannelOpenedEvent.Record()
}

func (m *Metrics) RecordChannelTimedOut() {
	m.channelTimedOutEvent.Record()
}

func (m *Metrics) RecordFrame() {
	m.frameAddedEvent.Record()
}

func (m *Metrics) RecordPeerUnban() {
	m.PeerUnbans.Inc()
}

func (m *Metrics) RecordIPUnban() {
	m.IPUnbans.Inc()
}

func (m *Metrics) RecordDial(allow bool) {
	if allow {
		m.Dials.WithLabelValues("true").Inc()
	} else {
		m.Dials.WithLabelValues("false").Inc()
	}
}

func (m *Metrics) RecordAccept(allow bool) {
	if allow {
		m.Accepts.WithLabelValues("true").Inc()
	} else {
		m.Accepts.WithLabelValues("false").Inc()
	}
}
func (m *Metrics) ReportProtocolVersions(local, engine, recommended, required params.ProtocolVersion) {
	m.ProtocolVersionDelta.WithLabelValues("local_recommended").Set(float64(local.Compare(recommended)))
	m.ProtocolVersionDelta.WithLabelValues("local_required").Set(float64(local.Compare(required)))
	m.ProtocolVersionDelta.WithLabelValues("engine_recommended").Set(float64(engine.Compare(recommended)))
	m.ProtocolVersionDelta.WithLabelValues("engine_required").Set(float64(engine.Compare(required)))
	m.ProtocolVersions.WithLabelValues(local.String(), engine.String(), recommended.String(), required.String()).Set(1)
}

type noopMetricer struct {
	metrics.NoopRPCMetrics
	event.NoopMetrics
}

var NoopMetrics Metricer = new(noopMetricer)

func (n *noopMetricer) RecordInfo(version string) {
}

func (n *noopMetricer) RecordUp() {
}

func (n *noopMetricer) SetDerivationIdle(status bool) {
}

func (m *noopMetricer) SetSequencerState(active bool) {
}

func (n *noopMetricer) RecordPipelineReset() {
}

func (n *noopMetricer) RecordSequencingError() {
}

func (n *noopMetricer) RecordPublishingError() {
}

func (n *noopMetricer) RecordDerivationError() {
}

func (n *noopMetricer) RecordReceivedUnsafePayload(payload *eth.ExecutionPayloadEnvelope) {
}

func (n *noopMetricer) RecordRef(layer string, name string, num uint64, timestamp uint64, h common.Hash) {
}

func (n *noopMetricer) RecordL1Ref(name string, ref eth.L1BlockRef) {
}

func (n *noopMetricer) RecordL2Ref(name string, ref eth.L2BlockRef) {
}

func (n *noopMetricer) RecordUnsafePayloadsBuffer(length uint64, memSize uint64, next eth.BlockID) {
}

func (n *noopMetricer) RecordDerivedBatches(batchType string) {
}

func (n *noopMetricer) CountSequencedTxsInBlock(txns int, deposits int) {
}

func (n *noopMetricer) RecordL1ReorgDepth(d uint64) {
}

func (n *noopMetricer) RecordSequencerInconsistentL1Origin(from eth.BlockID, to eth.BlockID) {
}

func (n *noopMetricer) RecordSequencerReset() {
}

func (n *noopMetricer) RecordGossipEvent(evType int32) {
}

func (n *noopMetricer) SetPeerScores(allScores []store.PeerScores) {
}

func (n *noopMetricer) IncPeerCount() {
}

func (n *noopMetricer) DecPeerCount() {
}

func (n *noopMetricer) IncStreamCount() {
}

func (n *noopMetricer) DecStreamCount() {
}

func (n *noopMetricer) RecordBandwidth(ctx context.Context, bwc *libp2pmetrics.BandwidthCounter) {
}

func (n *noopMetricer) RecordSequencerBuildingDiffTime(duration time.Duration) {
}

func (n *noopMetricer) RecordSequencerSealingTime(duration time.Duration) {
}

func (n *noopMetricer) Document() []metrics.DocumentedMetric {
	return nil
}

func (n *noopMetricer) ClientPayloadByNumberEvent(num uint64, resultCode byte, duration time.Duration) {
}

func (n *noopMetricer) ServerPayloadByNumberEvent(num uint64, resultCode byte, duration time.Duration) {
}

func (n *noopMetricer) PayloadsQuarantineSize(int) {
}

func (n *noopMetricer) RecordChannelInputBytes(int) {
}

func (n *noopMetricer) RecordHeadChannelOpened() {
}

func (n *noopMetricer) RecordChannelTimedOut() {
}

func (n *noopMetricer) RecordFrame() {
}

func (n *noopMetricer) RecordPeerUnban() {
}

func (n *noopMetricer) RecordIPUnban() {
}

func (n *noopMetricer) RecordDial(allow bool) {
}

func (n *noopMetricer) RecordAccept(allow bool) {
}
func (n *noopMetricer) ReportProtocolVersions(local, engine, recommended, required params.ProtocolVersion) {
}
