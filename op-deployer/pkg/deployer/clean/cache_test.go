package clean

import (
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/stretchr/testify/require"
)

func TestCacheCLI(t *testing.T) {
	tmpDir := t.TempDir()

	t.Cleanup(func() {
		require.NoError(t, os.RemoveAll(tmpDir))
	})

	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("test"), 0644))

	lgr := testlog.Logger(t, slog.LevelDebug)

	require.NoError(t, CleanCache(lgr, tmpDir))

	require.DirExists(t, tmpDir)
	files, err := os.ReadDir(tmpDir)
	require.NoError(t, err)
	require.Empty(t, files)
}

func TestCacheCLIE2E(t *testing.T) {
	tmpDirForCache := t.TempDir()
	tmpDirForBinary := t.TempDir()

	t.Cleanup(func() {
		require.NoError(t, os.RemoveAll(tmpDirForCache))
		require.NoError(t, os.RemoveAll(tmpDirForBinary))
	})

	require.NoError(t, os.WriteFile(filepath.Join(tmpDirForCache, "test.txt"), []byte("test"), 0644))

	binaryPath := filepath.Join(tmpDirForBinary, "op-deployer")
	cmd := exec.Command("go", "build", "-o", binaryPath, "../../../cmd/op-deployer/main.go")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", output)

	cmd = exec.Command(binaryPath, "--cache-dir", tmpDirForCache, "clean", "cache")
	output, err = cmd.CombinedOutput()
	require.NoError(t, err, "Failed to run clean cache command: %s", output)

	require.DirExists(t, tmpDirForCache)
	files, err := os.ReadDir(tmpDirForCache)
	require.NoError(t, err)
	require.Empty(t, files)
}
