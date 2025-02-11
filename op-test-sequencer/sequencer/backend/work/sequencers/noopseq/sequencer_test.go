package noopseq

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/seqtypes"
)

func TestNoopSequencer(t *testing.T) {
	logger := testlog.Logger(t, log.LevelDebug)
	id := seqtypes.SequencerID("foo")
	x := NewSequencer(id, logger)

	ctx := context.Background()
	require.NoError(t, x.Open(ctx))
	job := x.BuildJob()
	require.Nil(t, job)
	require.NoError(t, x.Seal(ctx))
	require.NoError(t, x.Prebuilt(ctx, nil))
	require.NoError(t, x.Sign(ctx))
	require.NoError(t, x.Commit(ctx))
	require.NoError(t, x.Publish(ctx))
	require.NoError(t, x.Next(ctx))
	err := x.Start(ctx, common.Hash{})
	require.ErrorIs(t, err, seqtypes.ErrSequencerInactive)
	_, err = x.Stop(ctx)
	require.ErrorIs(t, err, seqtypes.ErrSequencerInactive)

	require.NoError(t, x.Close())
	require.Equal(t, "noop-sequencer-foo", x.String())
	require.Equal(t, id, x.ID())
}
