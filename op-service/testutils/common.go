package testutils

import (
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/require"
)

func IsolatedTestDirWithAutoCleanup(t *testing.T) string {
	basePath := os.Getenv("TEST_ARTIFACTS_DIR")
	if basePath == "" {
		basePath = "./.tests"
	}
	dir := path.Join(basePath, t.Name())
	require.NoError(t, os.MkdirAll(dir, 0755))
	t.Cleanup(func() {
		require.NoError(t, os.RemoveAll(dir))
	})
	return dir
}
