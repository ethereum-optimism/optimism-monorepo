package builder

import (
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-test-sequencer/metrics"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/seqtypes"
)

type LocalBuilder struct {
	registry Registry
	id       seqtypes.BuilderID
	log      log.Logger
	m        metrics.Metricer
}

func (n *LocalBuilder) Attach(registry Registry) {
	n.registry = registry
}

func (l *LocalBuilder) NewJob(id seqtypes.JobID) (BuildJob, error) {
	if l.registry == nil {
		return nil, ErrNoRegistry
	}
	// TODO
	return nil, nil
}

func (l *LocalBuilder) Close() error {
	// TODO close all ongoing jobs
	// TODO close RPCs
	return nil
}

func (l *LocalBuilder) String() string {
	return l.id.String()
}

var _ Builder = (*LocalBuilder)(nil)

func NewLocalBuilder(log log.Logger, m metrics.Metricer) *LocalBuilder {
	return &LocalBuilder{
		log: log,
		m:   m,
	}
}
