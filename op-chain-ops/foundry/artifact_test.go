package foundry

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestArtifactJSON tests round-trip serialization of a foundry artifact for commonly used fields.
func TestArtifactJSON(t *testing.T) {
	artifact, err := ReadArtifact("testdata/forge-artifacts/Owned.sol/Owned.json")
	require.NoError(t, err)

	data, err := json.Marshal(artifact)
	require.NoError(t, err)

	file, err := os.ReadFile("testdata/forge-artifacts/Owned.sol/Owned.json")
	require.NoError(t, err)

	got := unmarshalIntoMap(t, data)
	expected := unmarshalIntoMap(t, file)

	require.JSONEq(t, marshal(t, got["bytecode"]), marshal(t, expected["bytecode"]))
	require.JSONEq(t, marshal(t, got["deployedBytecode"]), marshal(t, expected["deployedBytecode"]))
	require.JSONEq(t, marshal(t, got["abi"]), marshal(t, expected["abi"]))
	require.JSONEq(t, marshal(t, got["storageLayout"]), marshal(t, expected["storageLayout"]))
	require.JSONEq(t, marshal(t, got["metadata"]), marshal(t, expected["metadata"]))
}

func unmarshalIntoMap(t *testing.T, file []byte) map[string]any {
	var result map[string]any
	err := json.Unmarshal(file, &result)
	require.NoError(t, err)
	return result
}

func marshal(t *testing.T, a any) string {
	result, err := json.Marshal(a)
	require.NoError(t, err)
	return string(result)
}
