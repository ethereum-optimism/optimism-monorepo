package builder

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNoopBuilder(t *testing.T) {
	x := NoopBuilder{}

	_, err := x.Open(context.Background())
	require.ErrorIs(t, err, ErrNoBuild)

	err = x.Cancel(context.Background(), "123")
	require.ErrorIs(t, err, ErrNoBuild)

	_, err = x.Seal(context.Background(), "123")
	require.ErrorIs(t, err, ErrNoBuild)

	require.NoError(t, x.Close())
}
