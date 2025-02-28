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
	// the dir's existence should be handled by Download as well else it should be left to break
	t.Cleanup(func() {
		require.NoError(t, os.RemoveAll(dir))
	})
	return dir
}
