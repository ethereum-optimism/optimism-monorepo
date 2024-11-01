package testutil

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"runtime"
	"testing"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/artifacts"

	"github.com/ethereum-optimism/optimism/op-chain-ops/foundry"
	op_service "github.com/ethereum-optimism/optimism/op-service"
	"github.com/stretchr/testify/require"
)

func LocalArtifacts(t *testing.T) (*artifacts.Locator, foundry.StatDirFs) {
	_, testFilename, _, ok := runtime.Caller(0)
	require.Truef(t, ok, "failed to get test filename")
	monorepoDir, err := op_service.FindMonorepoRoot(testFilename)
	require.NoError(t, err)
	artifactsDir := path.Join(monorepoDir, "packages", "contracts-bedrock", "forge-artifacts")
	return ArtifactsFromURL(t, fmt.Sprintf("file://%s", artifactsDir))
}

func ArtifactsFromURL(t *testing.T, artifactsURLStr string) (*artifacts.Locator, foundry.StatDirFs) {
	artifactsURL, err := url.Parse(artifactsURLStr)
	require.NoError(t, err)
	loc := &artifacts.Locator{
		URL: artifactsURL,
	}

	artifactsFS, cleanupArtifacts, err := artifacts.Download(context.Background(), loc, artifacts.NoopDownloadProgressor)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = cleanupArtifacts()
	})

	return loc, artifactsFS
}
