package config

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work/builders/noopbuilder"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/seqtypes"
)

func TestEnsemble_Start(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		v := new(Ensemble)
		out, err := v.Start(context.Background(), nil)
		require.NoError(t, err)
		require.Empty(t, out.Builders())
		require.Empty(t, out.Signers())
		require.Empty(t, out.Committers())
		require.Empty(t, out.Publishers())
		require.Empty(t, out.Sequencers())
	})
	t.Run("noops", func(t *testing.T) {
		v := &Ensemble{
			Endpoints: nil,
			Builders: map[seqtypes.BuilderID]*BuilderEntry{
				"noop-builder": {
					Noop: &noopbuilder.Config{},
				},
			},
			Signers: map[seqtypes.SignerID]*SignerEntry{
				// TODO
			},
			Committers: map[seqtypes.CommitterID]*CommitterEntry{
				// TODO
			},
			Publishers: map[seqtypes.PublisherID]*PublisherEntry{
				// TODO
			},
			Sequencers: map[seqtypes.SequencerID]*SequencerEntry{
				// TODO
			},
		}
		out, err := v.Start(context.Background(), nil)
		require.NoError(t, err)
		require.Len(t, out.Builders(), 1)
		//require.Len(t, out.Signers(), 1)
		//require.Len(t, out.Committers(), 1)
		//require.Len(t, out.Publishers(), 1)
		//require.Len(t, out.Sequencers(), 1)
	})
}
