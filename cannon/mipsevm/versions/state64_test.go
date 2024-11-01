//go:build cannon64
// +build cannon64

package versions

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/cannon/mipsevm"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/multithreaded"
	"github.com/ethereum-optimism/optimism/op-service/serialize"
)

func TestNewFromState(t *testing.T) {
	t.Run("multithreaded64", func(t *testing.T) {
		actual, err := NewFromState(multithreaded.CreateEmptyState())
		require.NoError(t, err)
		require.IsType(t, &multithreaded.State{}, actual.FPVMState)
		require.Equal(t, VersionMultiThreaded64, actual.Version)
	})
}

func TestLoadStateFromFile(t *testing.T) {
	t.Run("Multithreaded64FromBinary", func(t *testing.T) {
		expected, err := NewFromState(multithreaded.CreateEmptyState())
		require.NoError(t, err)

		path := writeToFile(t, "state.bin.gz", expected)
		actual, err := LoadStateFromFile(path)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	})
}

func TestVersionsOtherThanZeroDoNotSupportJSON(t *testing.T) {
	tests := []struct {
		version     StateVersion
		createState func() mipsevm.FPVMState
	}{
		{VersionMultiThreaded64, func() mipsevm.FPVMState { return multithreaded.CreateEmptyState() }},
	}
	for _, test := range tests {
		test := test
		t.Run(test.version.String(), func(t *testing.T) {
			state, err := NewFromState(test.createState())
			require.NoError(t, err)

			dir := t.TempDir()
			path := filepath.Join(dir, "test.json")
			err = serialize.Write(path, state, 0o644)
			require.ErrorIs(t, err, ErrJsonNotSupported)
		})
	}
}

func writeToFile(t *testing.T, filename string, data serialize.Serializable) string {
	dir := t.TempDir()
	path := filepath.Join(dir, filename)
	require.NoError(t, serialize.Write(path, data, 0o644))
	return path
}
