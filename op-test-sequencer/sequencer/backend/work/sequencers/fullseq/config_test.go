package fullseq

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/metrics"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work/builders/noopbuilder"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work/committers/noopcommitter"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work/publishers/nooppublisher"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work/signers/noopsigner"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/seqtypes"
)

func TestConfig(t *testing.T) {
	logger := testlog.Logger(t, log.LevelDebug)
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
	cfg := &Config{
		ChainID:             eth.ChainIDFromUInt64(1),
		Builder:             builderID,
		Signer:              signerID,
		Committer:           committerID,
		Publisher:           publisherID,
		SequencerConfDepth:  2,
		SequencerEnabled:    true,
		SequencerStopped:    true,
		SequencerMaxSafeLag: 10,
	}
	opts := &work.StartOpts{
		Log:     logger,
		Metrics: &metrics.NoopMetrics{},
	}
	seq, err := cfg.Start(context.Background(), sequencerID, ensemble, opts)
	require.NoError(t, err)
	require.NoError(t, ensemble.AddSequencer(seq))
}
