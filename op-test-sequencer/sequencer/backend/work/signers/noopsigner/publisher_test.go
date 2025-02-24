package noopsigner

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/seqtypes"
)

func TestNoopSigner(t *testing.T) {
	logger := testlog.Logger(t, log.LevelDebug)
	id := seqtypes.SignerID("foo")
	x := NewSigner(id, logger)

	_, err := x.Sign(context.Background(), nil)
	require.NoError(t, err)

	require.NoError(t, x.Close())
	require.Equal(t, "noop-signer-foo", x.String())
	require.Equal(t, id, x.ID())
}
