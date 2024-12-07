package foundry

import (
	"os"
	"testing"
	"github.com/ethereum-optimism/optimism/op-chain-ops/srcmap"
	"github.com/stretchr/testify/require"
)

//go:generate ./testdata/srcmaps/generate.sh

func TestSourceMapFS(t *testing.T) {
	artifactFS := OpenArtifactsDir("./testdata/srcmaps/test-artifacts")
	exampleArtifact, err := artifactFS.ReadArtifact("SimpleStorage.sol", "SimpleStorage")
	require.NoError(t, err)
	srcFS := NewSourceMapFS(os.DirFS("./testdata/srcmaps"))
	srcMap, err := srcFS.SourceMap(exampleArtifact, "SimpleStorage")
	require.NoError(t, err)
	seenInfo := make(map[string]struct{})
	for i := range exampleArtifact.DeployedBytecode.Object {
		seenInfo[srcMap.FormattedInfo(uint64(i))] = struct{}{}
	}
	require.Contains(t, seenInfo, "src/SimpleStorage.sol:11:5")
	require.Contains(t, seenInfo, "src/StorageLibrary.sol:8:9")
}

func TestReadSourceIDs(t *testing.T) {
	// Setup the test environment
	srcFS := NewSourceMapFS(os.DirFS("./testdata/srcmaps"))

	// Define the test parameters
	srcPath := "src/SimpleStorage.sol"
	contract := "SimpleStorage"
	compilerVersion := "0.8.15"

	// Call the ReadSourceIDs function
	ids, err := srcFS.ReadSourceIDs(srcPath, contract, compilerVersion)

	// Assert no error occurred
	require.NoError(t, err)

	// Assert that the source IDs map is not empty
	require.NotEmpty(t, ids)

	// Check for specific expected mappings
	expectedPath := "src/SimpleStorage.sol"
	require.Contains(t, ids, srcmap.SourceID(0))
	require.Equal(t, expectedPath, ids[srcmap.SourceID(0)])
}
