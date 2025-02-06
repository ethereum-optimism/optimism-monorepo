package metrics

import (
	"io"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ethereum-optimism/optimism/op-service/httputil"
	"github.com/ethereum-optimism/optimism/op-service/sources/caching"

	contractMetrics "github.com/ethereum-optimism/optimism/op-challenger/game/fault/contracts/metrics"
	opmetrics "github.com/ethereum-optimism/optimism/op-service/metrics"
	txmetrics "github.com/ethereum-optimism/optimism/op-service/txmgr/metrics"
)

const Namespace = "op_challenger"

type Metricer interface {
	RecordInfo(version string)
	RecordUp()

	StartBalanceMetrics(l log.Logger, client *ethclient.Client, account common.Address) io.Closer

	// Record Tx metrics
	txmetrics.TxMetricer

	// Record cache metrics
	caching.Metrics

	// Record contract metrics
	contractMetrics.ContractMetricer

	// Record Vm metrics
	VmMetricer

	RecordActedL1Block(n uint64)

	RecordGameStep()
	RecordGameMove()
	RecordGameL2Challenge()
	RecordClaimResolutionTime(t float64)
	RecordGameActTime(t float64)

	RecordPreimageChallenged()
	RecordPreimageChallengeFailed()

	RecordBondClaimFailed()
	RecordBondClaimed(amount uint64)

	RecordGamesStatus(inProgress, defenderWon, challengerWon int)

	RecordGameUpdateScheduled()
	RecordGameUpdateCompleted()

	RecordLargePreimageCount(count int)

	IncActiveExecutors()
	DecActiveExecutors()
	IncIdleExecutors()
	DecIdleExecutors()

	ToTypedVmMetrics(vmType string) TypedVmMetricer
}

// Metrics implementation must implement RegistryMetricer to allow the metrics server to work.
var _ opmetrics.RegistryMetricer = (*Metrics)(nil)

type Metrics struct {
	ns       string
	registry *prometheus.Registry
	factory  opmetrics.Factory

	txmetrics.TxMetrics
	*opmetrics.CacheMetrics
	*contractMetrics.ContractMetrics
	*VmMetrics

	info *prometheus.GaugeVec
	up   prometheus.Gauge

	executors *prometheus.GaugeVec

	bondClaimFailures prometheus.Counter
	bondsClaimed      prometheus.Counter

	preimageChallenged      prometheus.Counter
	preimageChallengeFailed prometheus.Counter
	preimageCount           prometheus.Gauge

	highestActedL1Block prometheus.Gauge

	moves        prometheus.Counter
	steps        prometheus.Counter
	l2Challenges prometheus.Counter

	claimResolutionTime prometheus.Histogram
	gameActTime         prometheus.Histogram

	trackedGames  *prometheus.GaugeVec
	inflightGames prometheus.Gauge
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

		TxMetrics: txmetrics.MakeTxMetrics(Namespace, factory),

		CacheMetrics: opmetrics.NewCacheMetrics(factory, Namespace, "provider_cache", "Provider cache"),

		ContractMetrics: contractMetrics.MakeContractMetrics(Namespace, factory),

		VmMetrics: NewVmMetrics(Namespace, factory),

		info: factory.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "info",
			Help:      "Pseudo-metric tracking version and config info",
		}, []string{
			"version",
		}),
		up: factory.NewGauge(prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "up",
			Help:      "1 if the op-challenger has finished starting up",
		}),
		executors: factory.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "executors",
			Help:      "Number of active and idle executors",
		}, []string{
			"status",
		}),
		moves: factory.NewCounter(prometheus.CounterOpts{
			Namespace: Namespace,
			Name:      "moves",
			Help:      "Number of game moves made by the challenge agent",
		}),
		steps: factory.NewCounter(prometheus.CounterOpts{
			Namespace: Namespace,
			Name:      "steps",
			Help:      "Number of game steps made by the challenge agent",
		}),
		l2Challenges: factory.NewCounter(prometheus.CounterOpts{
			Namespace: Namespace,
			Name:      "l2_challenges",
			Help:      "Number of L2 challenges made by the challenge agent",
		}),
		claimResolutionTime: factory.NewHistogram(prometheus.HistogramOpts{
			Namespace: Namespace,
			Name:      "claim_resolution_time",
			Help:      "Time (in seconds) spent trying to resolve claims",
			Buckets:   []float64{.05, .1, .25, .5, 1, 2.5, 5, 7.5, 10},
		}),
		gameActTime: factory.NewHistogram(prometheus.HistogramOpts{
			Namespace: Namespace,
			Name:      "game_act_time",
			Help:      "Time (in seconds) spent acting on a game",
			Buckets: append(
				[]float64{1.0, 2.0, 5.0, 10.0},
				prometheus.ExponentialBuckets(30.0, 2.0, 14)...),
		}),
		bondClaimFailures: factory.NewCounter(prometheus.CounterOpts{
			Namespace: Namespace,
			Name:      "claim_failures",
			Help:      "Number of bond claims that failed",
		}),
		bondsClaimed: factory.NewCounter(prometheus.CounterOpts{
			Namespace: Namespace,
			Name:      "bonds",
			Help:      "Number of bonds claimed by the challenge agent",
		}),
		preimageChallenged: factory.NewCounter(prometheus.CounterOpts{
			Namespace: Namespace,
			Name:      "preimage_challenged",
			Help:      "Number of preimages challenged by the challenger",
		}),
		preimageChallengeFailed: factory.NewCounter(prometheus.CounterOpts{
			Namespace: Namespace,
			Name:      "preimage_challenge_failed",
			Help:      "Number of preimage challenges that failed",
		}),
		preimageCount: factory.NewGauge(prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "preimage_count",
			Help:      "Number of large preimage proposals being tracked by the challenger",
		}),
		trackedGames: factory.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "tracked_games",
			Help:      "Number of games being tracked by the challenger",
		}, []string{
			"status",
		}),
		highestActedL1Block: factory.NewGauge(prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "highest_acted_l1_block",
			Help:      "Highest L1 block acted on by the challenger",
		}),
		inflightGames: factory.NewGauge(prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "inflight_games",
			Help:      "Number of games being tracked by the challenger",
		}),
	}
}

func (m *Metrics) Start(host string, port int) (*httputil.HTTPServer, error) {
	return opmetrics.StartServer(m.registry, host, port)
}

func (m *Metrics) StartBalanceMetrics(
	l log.Logger,
	client *ethclient.Client,
	account common.Address,
) io.Closer {
	return opmetrics.LaunchBalanceMetrics(l, m.registry, m.ns, client, account)
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

func (m *Metrics) Document() []opmetrics.DocumentedMetric {
	return m.factory.Document()
}

func (m *Metrics) RecordGameMove() {
	m.moves.Add(1)
}

func (m *Metrics) RecordGameStep() {
	m.steps.Add(1)
}

func (m *Metrics) RecordGameL2Challenge() {
	m.l2Challenges.Add(1)
}

func (m *Metrics) RecordPreimageChallenged() {
	m.preimageChallenged.Add(1)
}

func (m *Metrics) RecordPreimageChallengeFailed() {
	m.preimageChallengeFailed.Add(1)
}

func (m *Metrics) RecordLargePreimageCount(count int) {
	m.preimageCount.Set(float64(count))
}

func (m *Metrics) RecordBondClaimFailed() {
	m.bondClaimFailures.Add(1)
}

func (m *Metrics) RecordBondClaimed(amount uint64) {
	m.bondsClaimed.Add(float64(amount))
}

func (m *Metrics) RecordClaimResolutionTime(t float64) {
	m.claimResolutionTime.Observe(t)
}

func (m *Metrics) RecordGameActTime(t float64) {
	m.gameActTime.Observe(t)
}

func (m *Metrics) IncActiveExecutors() {
	m.executors.WithLabelValues("active").Inc()
}

func (m *Metrics) DecActiveExecutors() {
	m.executors.WithLabelValues("active").Dec()
}

func (m *Metrics) IncIdleExecutors() {
	m.executors.WithLabelValues("idle").Inc()
}

func (m *Metrics) DecIdleExecutors() {
	m.executors.WithLabelValues("idle").Dec()
}

func (m *Metrics) RecordGamesStatus(inProgress, defenderWon, challengerWon int) {
	m.trackedGames.WithLabelValues("in_progress").Set(float64(inProgress))
	m.trackedGames.WithLabelValues("defender_won").Set(float64(defenderWon))
	m.trackedGames.WithLabelValues("challenger_won").Set(float64(challengerWon))
}

func (m *Metrics) RecordActedL1Block(n uint64) {
	m.highestActedL1Block.Set(float64(n))
}

func (m *Metrics) RecordGameUpdateScheduled() {
	m.inflightGames.Add(1)
}

func (m *Metrics) RecordGameUpdateCompleted() {
	m.inflightGames.Sub(1)
}

func (m *Metrics) ToTypedVmMetrics(vmType string) TypedVmMetricer {
	return NewTypedVmMetrics(m, vmType)
}
