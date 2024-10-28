package metrics

import (
	"fmt"
	"io"
	"math/big"
	"strings"
	"time"

	contractMetrics "github.com/ethereum-optimism/optimism/op-challenger/game/fault/contracts/metrics"
	"github.com/ethereum-optimism/optimism/op-service/sources/caching"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ethereum-optimism/optimism/op-service/httputil"
	opmetrics "github.com/ethereum-optimism/optimism/op-service/metrics"
)

const Namespace = "op_dispute_mon"

type ResolutionStatus uint8

const (
	// In progress
	CompleteMaxDuration ResolutionStatus = iota
	CompleteBeforeMaxDuration

	// Resolvable
	ResolvableMaxDuration
	ResolvableBeforeMaxDuration

	// Not resolvable
	InProgressMaxDuration
	InProgressBeforeMaxDuration
)

type CreditExpectation uint8

const (
	// Max Duration reached
	CreditBelowWithdrawable CreditExpectation = iota
	CreditEqualWithdrawable
	CreditAboveWithdrawable

	// Max Duration not reached
	CreditBelowNonWithdrawable
	CreditEqualNonWithdrawable
	CreditAboveNonWithdrawable
)

type GameAgreementStatus uint8

const (
	// In progress
	AgreeChallengerAhead GameAgreementStatus = iota
	DisagreeChallengerAhead
	AgreeDefenderAhead
	DisagreeDefenderAhead

	// Completed
	AgreeDefenderWins
	DisagreeDefenderWins
	AgreeChallengerWins
	DisagreeChallengerWins
)

type ClaimStatus struct {
	resolved     bool
	clockExpired bool
	firstHalf    bool
	resolvable   bool
}

func (s ClaimStatus) AsLabels() []string {
	labels := make([]string, 4)
	if s.resolved {
		labels[0] = "resolved"
	} else {
		labels[0] = "unresolved"
	}
	if s.clockExpired {
		labels[1] = "expired"
	} else {
		labels[1] = "not_expired"
	}
	if s.firstHalf {
		labels[2] = "first_half"
	} else {
		labels[2] = "second_half"
	}
	if s.resolvable {
		labels[3] = "resolvable"
	} else {
		labels[3] = "unresolvable"
	}
	return labels
}

func (s ClaimStatus) String() string {
	return strings.Join(s.AsLabels(), ", ")
}

type ClaimStatuses struct {
	statuses map[ClaimStatus]int
}

func (c *ClaimStatuses) RecordClaim(firstHalf, clockExpired, resolvable, resolved bool) {
	if c.statuses == nil {
		c.statuses = make(map[ClaimStatus]int)
	}
	c.statuses[NewClaimStatus(firstHalf, clockExpired, resolvable, resolved)]++
}

// ForEachStatus iterates through all possible statuses and calls the callback function with the status and count of
// claims. This ensures that statuses that have no claims counted against them are still considered to have 0 claims.
func (c *ClaimStatuses) ForEachStatus(callback func(status ClaimStatus, count int)) {
	allBools := []bool{true, false}
	for _, firstHalf := range allBools {
		for _, clockExpired := range allBools {
			for _, resolvable := range allBools {
				for _, resolved := range allBools {
					status := NewClaimStatus(firstHalf, clockExpired, resolvable, resolved)
					count := c.statuses[status]
					callback(status, count)
				}
			}
		}
	}
}

func NewClaimStatus(firstHalf, clockExpired, resolvable, resolved bool) ClaimStatus {
	return ClaimStatus{
		firstHalf:    firstHalf,
		clockExpired: clockExpired,
		resolvable:   resolvable,
		resolved:     resolved,
	}
}

type HonestActorData struct {
	PendingClaimCount int
	ValidClaimCount   int
	InvalidClaimCount int
	PendingBonds      *big.Int
	LostBonds         *big.Int
	WonBonds          *big.Int
}

type Metricer interface {
	RecordInfo(version string)
	RecordUp()

	RecordMonitorDuration(dur time.Duration)

	RecordFailedGames(count int)

	RecordHonestActorClaims(address common.Address, stats *HonestActorData)

	RecordGameResolutionStatus(status ResolutionStatus, count int)

	RecordCredit(expectation CreditExpectation, count int)

	RecordHonestWithdrawableAmounts(map[common.Address]*big.Int)

	RecordClaims(statuses *ClaimStatuses)

	RecordWithdrawalRequests(delayedWeth common.Address, matches bool, count int)

	RecordOutputFetchTime(timestamp float64)

	RecordGameAgreement(status GameAgreementStatus, count int)

	RecordLatestValidProposalL2Block(latestValid uint64)

	RecordLatestProposals(latestValid, latestInvalid uint64)

	RecordIgnoredGames(count int)

	RecordBondCollateral(addr common.Address, required, available *big.Int)

	RecordL2Challenges(agreement bool, count int)

	RecordOldestGameUpdateTime(t time.Time)

	caching.Metrics
	contractMetrics.ContractMetricer
}

// Metrics implementation must implement RegistryMetricer to allow the metrics server to work.
var _ opmetrics.RegistryMetricer = (*Metrics)(nil)

type Metrics struct {
	ns       string
	registry *prometheus.Registry
	factory  opmetrics.Factory

	*opmetrics.CacheMetrics
	*contractMetrics.ContractMetrics

	monitorDuration prometheus.Histogram

	resolutionStatus prometheus.GaugeVec

	claims prometheus.GaugeVec

	honestActorClaims prometheus.GaugeVec
	honestActorBonds  prometheus.GaugeVec

	withdrawalRequests prometheus.GaugeVec

	info prometheus.GaugeVec
	up   prometheus.Gauge

	credits                   prometheus.GaugeVec
	honestWithdrawableAmounts prometheus.GaugeVec

	lastOutputFetch      prometheus.Gauge
	oldestGameUpdateTime prometheus.Gauge

	gamesAgreement             prometheus.GaugeVec
	latestValidProposalL2Block prometheus.Gauge
	latestProposals            prometheus.GaugeVec
	ignoredGames               prometheus.Gauge
	failedGames                prometheus.Gauge
	l2Challenges               prometheus.GaugeVec

	requiredCollateral  prometheus.GaugeVec
	availableCollateral prometheus.GaugeVec
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

		CacheMetrics:    opmetrics.NewCacheMetrics(factory, Namespace, "provider_cache", "Provider cache"),
		ContractMetrics: contractMetrics.MakeContractMetrics(Namespace, factory),

		info: *factory.NewGaugeVec(prometheus.GaugeOpts{
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
		monitorDuration: factory.NewHistogram(prometheus.HistogramOpts{
			Namespace: Namespace,
			Name:      "monitor_duration_seconds",
			Help:      "Time taken to complete a cycle of updating metrics for all games",
			Buckets:   []float64{10, 30, 60, 120, 180, 300, 600},
		}),
		lastOutputFetch: factory.NewGauge(prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "last_output_fetch",
			Help:      "Timestamp of the last output fetch",
		}),
		oldestGameUpdateTime: factory.NewGauge(prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "oldest_game_update_time",
			Help: "Timestamp the least recently updated game " +
				"or the time of the last update cycle if there were no games in the monitoring window",
		}),
		honestActorClaims: *factory.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "honest_actor_claims",
			Help:      "Total number of claims from an honest actor",
		}, []string{
			"honest_actor_address",
			"state",
		}),
		honestActorBonds: *factory.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "honest_actor_bonds",
			Help:      "Sum of bonds posted, won and lost by an honest actor",
		}, []string{
			"honest_actor_address",
			"state",
		}),
		resolutionStatus: *factory.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "resolution_status",
			Help:      "Number of games categorised by whether the game is complete and whether the maximum duration has been reached",
		}, []string{
			"completion",
			"max_duration",
		}),
		credits: *factory.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "credits",
			Help:      "Cumulative credits",
		}, []string{
			"credit",
			"withdrawable",
		}),
		honestWithdrawableAmounts: *factory.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "honest_actor_pending_withdrawals",
			Help:      "Current amount of withdrawable ETH for an honest actor",
		}, []string{
			"actor",
		}),
		claims: *factory.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "claims",
			Help:      "Claims broken down by whether they were resolved, whether the clock expired, and the game time period",
		}, []string{
			"resolved",
			"clock",
			"game_time_period",
			"resolvable",
		}),
		withdrawalRequests: *factory.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "withdrawal_requests",
			Help:      "Number of withdrawal requests categorised by the source DelayedWETH contract and whether the withdrawal request amount matches or diverges from its fault dispute game credits",
		}, []string{
			"delayedWETH",
			"credits",
		}),
		gamesAgreement: *factory.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "games_agreement",
			Help:      "Number of games broken down by whether the result agrees with the reference node",
		}, []string{
			"status",
			"completion",
			"result_correctness",
			"root_agreement",
		}),
		latestValidProposalL2Block: factory.NewGauge(prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "latest_valid_proposal_l2_block",
			Help:      "L2 block number proposed by the latest game with a valid root claim",
		}),
		latestProposals: *factory.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "latest_proposal",
			Help:      "Timestamp of the most recent game with a valid or invalid root claim in unix seconds",
		},
			[]string{"root_agreement"}),
		ignoredGames: factory.NewGauge(prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "ignored_games",
			Help:      "Number of games present in the game window but ignored via config",
		}),
		requiredCollateral: *factory.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "bond_collateral_required",
			Help:      "Required collateral (ETH) to cover outstanding bonds and credits",
		}, []string{
			// Address of the DelayedWETH contract in use. This is a limited set as only permissioned actors can deploy
			// additional DelayedWETH contracts to be used by dispute games
			"delayedWETH",
			"balance",
		}),
		failedGames: factory.NewGauge(prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "failed_games",
			Help:      "Number of games present in the game window but failed to be monitored",
		}),
		availableCollateral: *factory.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "bond_collateral_available",
			Help:      "Available collateral (ETH) to cover outstanding bonds and credits",
		}, []string{
			// Address of the DelayedWETH contract in use. This is a limited set as only permissioned actors can deploy
			// additional DelayedWETH contracts to be used by dispute games
			"delayedWETH",
			"balance",
		}),
		l2Challenges: *factory.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "l2_block_challenges",
			Help:      "Number of games where the L2 block number has been successfully challenged",
		}, []string{
			// Agreement with the root claim, not the actual l2 block number challenge.
			// An l2 block number challenge with an agreement means the challenge was invalid.
			"root_agreement",
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
	prometheus.MustRegister()
	m.up.Set(1)
}

func (m *Metrics) RecordMonitorDuration(dur time.Duration) {
	m.monitorDuration.Observe(dur.Seconds())
}

func (m *Metrics) RecordHonestActorClaims(address common.Address, stats *HonestActorData) {
	m.honestActorClaims.WithLabelValues(address.Hex(), "pending").Set(float64(stats.PendingClaimCount))
	m.honestActorClaims.WithLabelValues(address.Hex(), "invalid").Set(float64(stats.InvalidClaimCount))
	m.honestActorClaims.WithLabelValues(address.Hex(), "valid").Set(float64(stats.ValidClaimCount))

	m.honestActorBonds.WithLabelValues(address.Hex(), "pending").Set(weiToEther(stats.PendingBonds))
	m.honestActorBonds.WithLabelValues(address.Hex(), "lost").Set(weiToEther(stats.LostBonds))
	m.honestActorBonds.WithLabelValues(address.Hex(), "won").Set(weiToEther(stats.WonBonds))
}

func (m *Metrics) RecordGameResolutionStatus(status ResolutionStatus, count int) {
	asLabels := func(status ResolutionStatus) []string {
		switch status {
		case CompleteMaxDuration:
			return []string{"complete", "max_duration"}
		case CompleteBeforeMaxDuration:
			return []string{"complete", "before_max_duration"}
		case ResolvableMaxDuration:
			return []string{"resolvable", "max_duration"}
		case ResolvableBeforeMaxDuration:
			return []string{"resolvable", "before_max_duration"}
		case InProgressMaxDuration:
			return []string{"in_progress", "max_duration"}
		case InProgressBeforeMaxDuration:
			return []string{"in_progress", "before_max_duration"}
		default:
			panic(fmt.Errorf("unknown resolution status: %v", status))
		}
	}
	m.resolutionStatus.WithLabelValues(asLabels(status)...).Set(float64(count))
}

func (m *Metrics) RecordCredit(expectation CreditExpectation, count int) {
	asLabels := func(expectation CreditExpectation) []string {
		switch expectation {
		case CreditBelowWithdrawable:
			return []string{"below", "withdrawable"}
		case CreditEqualWithdrawable:
			return []string{"expected", "withdrawable"}
		case CreditAboveWithdrawable:
			return []string{"above", "withdrawable"}
		case CreditBelowNonWithdrawable:
			return []string{"below", "non_withdrawable"}
		case CreditEqualNonWithdrawable:
			return []string{"expected", "non_withdrawable"}
		case CreditAboveNonWithdrawable:
			return []string{"above", "non_withdrawable"}
		default:
			panic(fmt.Errorf("unknown credit expectation: %v", expectation))
		}
	}
	m.credits.WithLabelValues(asLabels(expectation)...).Set(float64(count))
}

func (m *Metrics) RecordHonestWithdrawableAmounts(amounts map[common.Address]*big.Int) {
	for addr, amount := range amounts {
		m.honestWithdrawableAmounts.WithLabelValues(addr.Hex()).Set(weiToEther(amount))
	}
}

func (m *Metrics) RecordClaims(statuses *ClaimStatuses) {
	statuses.ForEachStatus(func(status ClaimStatus, count int) {
		m.claims.WithLabelValues(status.AsLabels()...).Set(float64(count))
	})
}

func (m *Metrics) RecordWithdrawalRequests(delayedWeth common.Address, matches bool, count int) {
	credits := "matching"
	if !matches {
		credits = "divergent"
	}
	m.withdrawalRequests.WithLabelValues(delayedWeth.Hex(), credits).Set(float64(count))
}

func (m *Metrics) Document() []opmetrics.DocumentedMetric {
	return m.factory.Document()
}

func (m *Metrics) RecordOutputFetchTime(timestamp float64) {
	m.lastOutputFetch.Set(timestamp)
}

func (m *Metrics) RecordOldestGameUpdateTime(t time.Time) {
	m.oldestGameUpdateTime.Set(float64(t.Unix()))
}

func (m *Metrics) RecordGameAgreement(status GameAgreementStatus, count int) {
	m.gamesAgreement.WithLabelValues(labelValuesFor(status)...).Set(float64(count))
}

func (m *Metrics) RecordLatestValidProposalL2Block(latestValid uint64) {
	m.latestValidProposalL2Block.Set(float64(latestValid))
}

func (m *Metrics) RecordLatestProposals(latestValid, latestInvalid uint64) {
	m.latestProposals.WithLabelValues("agree").Set(float64(latestValid))
	m.latestProposals.WithLabelValues("disagree").Set(float64(latestInvalid))
}

func (m *Metrics) RecordIgnoredGames(count int) {
	m.ignoredGames.Set(float64(count))
}

func (m *Metrics) RecordFailedGames(count int) {
	m.failedGames.Set(float64(count))
}

func (m *Metrics) RecordBondCollateral(addr common.Address, required, available *big.Int) {
	balanceLabel := "sufficient"
	zeroBalanceLabel := "insufficient"
	if required.Cmp(available) > 0 {
		balanceLabel = "insufficient"
		zeroBalanceLabel = "sufficient"
	}
	m.requiredCollateral.WithLabelValues(addr.Hex(), balanceLabel).Set(weiToEther(required))
	m.availableCollateral.WithLabelValues(addr.Hex(), balanceLabel).Set(weiToEther(available))

	// If the balance is sufficient, make sure the insufficient label is zeroed out and vice versa.
	m.requiredCollateral.WithLabelValues(addr.Hex(), zeroBalanceLabel).Set(0)
	m.availableCollateral.WithLabelValues(addr.Hex(), zeroBalanceLabel).Set(0)
}

func (m *Metrics) RecordL2Challenges(agreement bool, count int) {
	agree := "disagree"
	if agreement {
		agree = "agree"
	}
	m.l2Challenges.WithLabelValues(agree).Set(float64(count))
}

const (
	inProgress = true
	correct    = true
	agree      = true
)

func labelValuesFor(status GameAgreementStatus) []string {
	asStrings := func(status string, inProgress, correct, agree bool) []string {
		inProgressStr := "in_progress"
		if !inProgress {
			inProgressStr = "complete"
		}
		correctStr := "correct"
		if !correct {
			correctStr = "incorrect"
		}
		agreeStr := "agree"
		if !agree {
			agreeStr = "disagree"
		}
		return []string{status, inProgressStr, correctStr, agreeStr}
	}
	switch status {
	case AgreeChallengerAhead:
		return asStrings("agree_challenger_ahead", inProgress, !correct, agree)
	case DisagreeChallengerAhead:
		return asStrings("disagree_challenger_ahead", inProgress, correct, !agree)
	case AgreeDefenderAhead:
		return asStrings("agree_defender_ahead", inProgress, correct, agree)
	case DisagreeDefenderAhead:
		return asStrings("disagree_defender_ahead", inProgress, !correct, !agree)

	// Completed
	case AgreeDefenderWins:
		return asStrings("agree_defender_wins", !inProgress, correct, agree)
	case DisagreeDefenderWins:
		return asStrings("disagree_defender_wins", !inProgress, !correct, !agree)
	case AgreeChallengerWins:
		return asStrings("agree_challenger_wins", !inProgress, !correct, agree)
	case DisagreeChallengerWins:
		return asStrings("disagree_challenger_wins", !inProgress, correct, !agree)

	default:
		panic(fmt.Errorf("unknown game agreement status: %v", status))
	}
}

// weiToEther divides the wei value by 10^18 to get a number in ether as a float64
func weiToEther(wei *big.Int) float64 {
	num := new(big.Rat).SetInt(wei)
	denom := big.NewRat(params.Ether, 1)
	num = num.Quo(num, denom)
	f, _ := num.Float64()
	return f
}
