package work_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/metrics"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work/builders/noopbuilder"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work/committers/noopcommitter"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work/publishers/nooppublisher"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work/sequencers/noopseq"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work/signers/noopsigner"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/seqtypes"
)

func TestEnsemble(t *testing.T) {
	logger := testlog.Logger(t, log.LevelDebug)
	builderID := seqtypes.BuilderID("test-builder")
	signerID := seqtypes.SignerID("test-signer")
	committerID := seqtypes.CommitterID("test-committer")
	publisherID := seqtypes.PublisherID("test-publisher")
	sequencerID := seqtypes.SequencerID("test-sequencer")
	ensemble := &work.Ensemble{}
	require.NoError(t, ensemble.AddBuilder(noopbuilder.NewBuilder(builderID)))
	require.ErrorIs(t, ensemble.AddBuilder(noopbuilder.NewBuilder(builderID)), work.ErrAlreadyExists)
	require.NoError(t, ensemble.AddSigner(noopsigner.NewSigner(signerID, logger)))
	require.ErrorIs(t, ensemble.AddSigner(noopsigner.NewSigner(signerID, logger)), work.ErrAlreadyExists)
	require.NoError(t, ensemble.AddCommitter(noopcommitter.NewCommitter(committerID, logger)))
	require.ErrorIs(t, ensemble.AddCommitter(noopcommitter.NewCommitter(committerID, logger)), work.ErrAlreadyExists)
	require.NoError(t, ensemble.AddPublisher(nooppublisher.NewPublisher(publisherID, logger)))
	require.ErrorIs(t, ensemble.AddPublisher(nooppublisher.NewPublisher(publisherID, logger)), work.ErrAlreadyExists)
	require.NoError(t, ensemble.AddSequencer(noopseq.NewSequencer(sequencerID, logger)))
	require.ErrorIs(t, ensemble.AddSequencer(noopseq.NewSequencer(sequencerID, logger)), work.ErrAlreadyExists)

	require.NotNil(t, ensemble.Builder(builderID))
	require.NotNil(t, ensemble.Signer(signerID))
	require.NotNil(t, ensemble.Committer(committerID))
	require.NotNil(t, ensemble.Publisher(publisherID))
	require.NotNil(t, ensemble.Sequencer(sequencerID))

	builderID2 := seqtypes.BuilderID("test-builder2")
	signerID2 := seqtypes.SignerID("test-signer2")
	committerID2 := seqtypes.CommitterID("test-committer2")
	publisherID2 := seqtypes.PublisherID("test-publisher2")
	sequencerID2 := seqtypes.SequencerID("test-sequencer2")
	require.NoError(t, ensemble.AddBuilder(noopbuilder.NewBuilder(builderID2)))
	require.NoError(t, ensemble.AddSigner(noopsigner.NewSigner(signerID2, logger)))
	require.NoError(t, ensemble.AddCommitter(noopcommitter.NewCommitter(committerID2, logger)))
	require.NoError(t, ensemble.AddPublisher(nooppublisher.NewPublisher(publisherID2, logger)))
	require.NoError(t, ensemble.AddSequencer(noopseq.NewSequencer(sequencerID2, logger)))

	require.Len(t, ensemble.Builders(), 2)
	require.Len(t, ensemble.Signers(), 2)
	require.Len(t, ensemble.Committers(), 2)
	require.Len(t, ensemble.Publishers(), 2)
	require.Len(t, ensemble.Sequencers(), 2)

	starter, err := ensemble.Load(context.Background())
	require.NoError(t, err)
	require.Equal(t, starter, ensemble, "can load in-place, to skip pre-config phase")

	self, err := ensemble.Start(context.Background(), &work.StartOpts{
		Log:     logger,
		Metrics: metrics.NoopMetrics{},
	})
	require.NoError(t, err)
	require.Equal(t, self, ensemble, "can start in-place, to skip config phase")

	require.NoError(t, ensemble.Close())
}
