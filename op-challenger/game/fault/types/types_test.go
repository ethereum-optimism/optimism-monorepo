package types

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewPreimageOracleData(t *testing.T) {
	t.Run("LocalData", func(t *testing.T) {
		data := NewPreimageOracleData([]byte{1, 2, 3}, []byte{4, 5, 6}, 7)
		require.True(t, data.IsLocal)
		require.Equal(t, []byte{1, 2, 3}, data.OracleKey)
		require.Equal(t, []byte{4, 5, 6}, data.GetPreimageWithSize())
		require.Equal(t, uint32(7), data.OracleOffset)
	})

	t.Run("GlobalData", func(t *testing.T) {
		data := NewPreimageOracleData([]byte{0, 2, 3}, []byte{4, 5, 6}, 7)
		require.False(t, data.IsLocal)
		require.Equal(t, []byte{0, 2, 3}, data.OracleKey)
		require.Equal(t, []byte{4, 5, 6}, data.GetPreimageWithSize())
		require.Equal(t, uint32(7), data.OracleOffset)
	})
}

func TestIsRootPosition(t *testing.T) {
	tests := []struct {
		name     string
		position Position
		expected bool
	}{
		{
			name:     "ZeroRoot",
			position: NewPositionFromGIndex(big.NewInt(0)),
			expected: true,
		},
		{
			name:     "ValidRoot",
			position: NewPositionFromGIndex(big.NewInt(1)),
			expected: true,
		},
		{
			name:     "NotRoot",
			position: NewPositionFromGIndex(big.NewInt(2)),
			expected: false,
		},
		{
			// Mostly to avoid nil dereferences in tests which may not set a real Position
			name:     "DefaultValue",
			position: Position{},
			expected: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.expected, test.position.IsRootPosition())
		})
	}
}

func TestKnownGameTypeForEveryTraceType(t *testing.T) {
	for _, traceType := range TraceTypes {
		traceType := traceType
		t.Run(traceType.String(), func(t *testing.T) {
			require.NotEqual(t, UnknownGameType, traceType.GameType())
		})
	}
}

func TestKnownStringForUtilisedGameType(t *testing.T) {
	for _, traceType := range TraceTypes {
		traceType := traceType
		t.Run(traceType.String(), func(t *testing.T) {
			require.NotContains(t, traceType.GameType().String(), "invalid")
		})
	}
}
