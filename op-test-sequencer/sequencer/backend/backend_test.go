package backend

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/metrics"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work/builders/noopbuilder"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work/committers/noopcommitter"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work/publishers/nooppublisher"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work/sequencers/fullseq"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work/signers/noopsigner"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/seqtypes"
)

type noopRouter struct {
	log log.Logger
}

func (n *noopRouter) AddRPC(route string) error {
	n.log.Debug("Adding RPC route", "route", route)
	return nil
}

func (n noopRouter) AddAPIToRPC(route string, api rpc.API) error {
	n.log.Debug("Adding API on route",
		"route", route,
		"namespace", api.Namespace,
		"handler", fmt.Sprintf("%T", api.Service))
	return nil
}

var _ APIRouter = (*noopRouter)(nil)

func TestBackend(t *testing.T) {
	logger := testlog.Logger(t, log.LevelDebug)
	m := &metrics.NoopMetrics{}
	builderID := seqtypes.BuilderID("test-builder")
	signerID := seqtypes.SignerID("test-signer")
	committerID := seqtypes.CommitterID("test-committer")
	publisherID := seqtypes.PublisherID("test-publisher")
	sequencerID := seqtypes.SequencerID("test-sequencer")
	ensemble := &work.Ensemble{}
	require.NoError(t, ensemble.AddBuilder(noopbuilder.NewBuilder(builderID)))
	require.NoError(t, ensemble.AddSigner(noopsigner.NewSigner(signerID, logger)))
	require.NoError(t, ensemble.AddCommitter(noopcommitter.NewCommitter(committerID, logger)))
	require.NoError(t, ensemble.AddPublisher(nooppublisher.NewPublisher(publisherID, logger)))
	seqCfg := &fullseq.Config{
		ChainID:             eth.ChainIDFromUInt64(123),
		Builder:             builderID,
		Signer:              signerID,
		Committer:           committerID,
		Publisher:           publisherID,
		SequencerConfDepth:  0,
		SequencerEnabled:    false,
		SequencerStopped:    false,
		SequencerMaxSafeLag: 0,
	}
	sequencer, err := seqCfg.Start(context.Background(), sequencerID, ensemble, &work.StartOpts{
		Log:     logger,
		Metrics: m,
	})
	require.NoError(t, err)
	require.NoError(t, ensemble.AddSequencer(sequencer))

	router := &noopRouter{log: logger}
	b := NewBackend(logger, m, ensemble, router)
	require.NoError(t, b.Start(context.Background()))
	require.ErrorIs(t, b.Start(context.Background()), seqtypes.ErrBackendAlreadyStarted)

	result, err := b.Hello(context.Background(), "alice")
	require.NoError(t, err)
	require.Contains(t, result, "alice")

	_, err = b.CreateJob(context.Background(), "not there", nil)
	require.ErrorIs(t, err, seqtypes.ErrUnknownBuilder)

	job, err := b.CreateJob(context.Background(), builderID, nil)
	require.NoError(t, err)

	_, err = job.Seal(context.Background())
	require.ErrorIs(t, err, noopbuilder.ErrNoBuild)

	require.Equal(t, job, b.GetJob(job.ID()))

	require.NoError(t, b.Stop(context.Background()))
	require.ErrorIs(t, b.Stop(context.Background()), seqtypes.ErrBackendInactive)
	require.ErrorIs(t, b.Start(context.Background()), seqtypes.ErrBackendAlreadyStarted, "no restarts")

	_, err = b.CreateJob(context.Background(), builderID, nil)
	require.ErrorIs(t, err, seqtypes.ErrBackendInactive)

	require.Zero(t, b.jobs.Len())
}
