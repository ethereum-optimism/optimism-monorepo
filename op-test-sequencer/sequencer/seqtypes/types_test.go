package seqtypes

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIDs(t *testing.T) {
	testID[*genericID](t, func() *genericID {
		return new(genericID)
	})
	testID[*BuilderID](t, func() *BuilderID {
		return new(BuilderID)
	})
	testID[*SignerID](t, func() *SignerID {
		return new(SignerID)
	})
	testID[*CommitterID](t, func() *CommitterID {
		return new(CommitterID)
	})
	testID[*PublisherID](t, func() *PublisherID {
		return new(PublisherID)
	})
	testID[*SequencerID](t, func() *SequencerID {
		return new(SequencerID)
	})
	testID[*BuildJobID](t, func() *BuildJobID {
		return new(BuildJobID)
	})
}

type id interface {
	String() string
	MarshalText() ([]byte, error)
	UnmarshalText(data []byte) error
}

func testID[K id](t *testing.T, alloc func() K) {
	var x K
	name := fmt.Sprintf("%T", x)
	t.Run(name, func(t *testing.T) {
		v := alloc()
		require.Equal(t, "", v.String())
		out, err := v.MarshalText()
		require.NoError(t, err)
		require.Len(t, out, 0)
		v = alloc()
		require.NoError(t, v.UnmarshalText([]byte("123abc")))
		require.Equal(t, "123abc", v.String())
		out, err = v.MarshalText()
		require.NoError(t, err)
		require.Equal(t, "123abc", string(out))

		require.NoError(t, v.UnmarshalText(bytes.Repeat([]byte{'1'}, maxIDLength)))
		require.ErrorIs(t, v.UnmarshalText(bytes.Repeat([]byte{'1'}, maxIDLength+1)), ErrInvalidID)
	})
}

func TestRandomJobID(t *testing.T) {
	a := RandomJobID()
	b := RandomJobID()
	require.NotEqual(t, a, b)
}
