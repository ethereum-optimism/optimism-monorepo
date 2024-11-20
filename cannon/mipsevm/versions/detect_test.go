package versions

import (
	"embed"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/cannon/mipsevm/arch"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/multithreaded"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/singlethreaded"
	"github.com/ethereum-optimism/optimism/op-service/ioutil"
)

const statesPath = "testdata/states"

//go:embed testdata/states
var historicStates embed.FS

func TestDetectVersion_fromFile(t *testing.T) {
	testDetection := func(t *testing.T, version StateVersion, ext string) {
		filename := strconv.Itoa(int(version)) + ext
		dir := t.TempDir()
		path := filepath.Join(dir, filename)
		in, err := historicStates.ReadFile(filepath.Join(statesPath, filename))
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(path, in, 0o644))

		detectedVersion, err := DetectVersion(path)
		require.NoError(t, err)
		require.Equal(t, version, detectedVersion)
	}
	// Iterate all known versions to ensure we have a test case to detect every state version
	for _, version := range StateVersionTypes {
		version := version
		t.Run(version.String(), func(t *testing.T) {
			testDetection(t, version, ".bin.gz")
		})

		if version == VersionSingleThreaded {
			t.Run(version.String()+".json", func(t *testing.T) {
				testDetection(t, version, ".json")
			})
		}
	}
}

// Check that the latest supported versions write new states in a way that is detected correctly
func TestDetectVersion_singleThreadedBinary(t *testing.T) {
	targetVersion := VersionSingleThreaded2
	if !arch.IsMips32 {
		t.Skip("Single-threaded states are not supported for 64-bit VMs")
	}

	state, err := NewFromState(singlethreaded.CreateEmptyState())
	require.NoError(t, err)
	path := writeToFile(t, "state.bin.gz", state)
	version, err := DetectVersion(path)
	require.NoError(t, err)
	require.Equal(t, targetVersion, version)
}

func TestDetectVersion_multiThreadedBinary(t *testing.T) {
	targetVersion := VersionMultiThreaded
	if !arch.IsMips32 {
		targetVersion = VersionMultiThreaded64
	}

	state, err := NewFromState(multithreaded.CreateEmptyState())
	require.NoError(t, err)
	path := writeToFile(t, "state.bin.gz", state)
	version, err := DetectVersion(path)
	require.NoError(t, err)
	require.Equal(t, targetVersion, version)
}

func TestDetectVersion_invalid(t *testing.T) {
	t.Run("bad gzip", func(t *testing.T) {
		dir := t.TempDir()
		filename := "state.bin.gz"
		path := filepath.Join(dir, filename)
		require.NoError(t, os.WriteFile(path, []byte("ekans"), 0o644))

		_, err := DetectVersion(path)
		require.ErrorContains(t, err, "failed to open file")
	})

	t.Run("unknown version", func(t *testing.T) {
		dir := t.TempDir()
		filename := "state.bin.gz"
		path := filepath.Join(dir, filename)
		const badVersion = 0xFF
		err := ioutil.WriteCompressedBytes(path, []byte{badVersion}, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		require.NoError(t, err)

		_, err = DetectVersion(path)
		require.ErrorIs(t, err, ErrUnknownVersion)
	})
}
