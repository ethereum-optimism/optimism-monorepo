package metrics

import (
	"io"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"

	opmetrics "github.com/ethereum-optimism/optimism/op-service/metrics"
	txmetrics "github.com/ethereum-optimism/optimism/op-service/txmgr/metrics"
)

const Namespace = "op_dripper"

var _ opmetrics.RegistryMetricer = (*Metrics)(nil)

type Metricer interface {
	RecordInfo(version string)
	RecordUp()

	opmetrics.RefMetricer
	opmetrics.RPCMetricer
	txmetrics.TxMetricer

	StartBalanceMetrics(l log.Logger, client *ethclient.Client, account common.Address) io.Closer

	RecordDripExecuted(name string)
}

type Metrics struct {
	ns       string
	registry *prometheus.Registry
	factory  opmetrics.Factory

	opmetrics.RefMetrics
	opmetrics.RPCMetrics
	txmetrics.TxMetrics

	info  prometheus.GaugeVec
	drips prometheus.GaugeVec
	up    prometheus.Gauge
}

var _ Metricer = (*Metrics)(nil)

func NewMetrics(procName string) *Metrics {
	if procName == "" {
		procName = "default"
	}
	ns := Namespace + "_" + procName

	registry := opmetrics.NewRegistry()
	factory := opmetrics.With(registry)

	return &Metrics{
		ns:       ns,
		registry: registry,
		factory:  factory,

		RefMetrics: opmetrics.MakeRefMetrics(ns, factory),
		RPCMetrics: opmetrics.MakeRPCMetrics(ns, factory),
		TxMetrics:  txmetrics.MakeTxMetrics(ns, factory),

		info: *factory.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns,
			Name:      "info",
			Help:      "Information about the dripper",
		}, []string{
			"version",
		}),
		drips: *factory.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns,
			Name:      "drips",
			Help:      "Drips executed",
		}, []string{
			"name",
		}),
		up: factory.NewGauge(prometheus.GaugeOpts{
			Namespace: ns,
			Name:      "up",
			Help:      "1 if the op-dripper has finished starting up",
		}),
	}
}

func (m *Metrics) Registry() *prometheus.Registry {
	return m.registry
}

func (m *Metrics) StartBalanceMetrics(l log.Logger, client *ethclient.Client, account common.Address) io.Closer {
	return opmetrics.LaunchBalanceMetrics(l, m.registry, m.ns, client, account)
}

func (m *Metrics) RecordInfo(version string) {
	m.info.WithLabelValues(version).Set(1)
}

func (m *Metrics) RecordUp() {
	m.up.Set(1)
}

func (m *Metrics) RecordDripExecuted(name string) {
	m.drips.WithLabelValues(name).Inc()
}

func (m *Metrics) Document() []opmetrics.DocumentedMetric {
	return m.factory.Document()
}
