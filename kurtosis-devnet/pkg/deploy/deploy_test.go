package deploy

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis"
	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis/sources/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockDeployerForTest implements the deployer interface for testing
type mockDeployerForTest struct {
	baseDir string
}

func (m *mockDeployerForTest) Deploy(ctx context.Context, input io.Reader) (*spec.EnclaveSpec, error) {
	// Create a mock env.json file
	envPath := filepath.Join(m.baseDir, "env.json")
	mockEnv := map[string]interface{}{
		"test": "value",
	}
	data, err := json.Marshal(mockEnv)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(envPath, data, 0644); err != nil {
		return nil, err
	}
	return &spec.EnclaveSpec{}, nil
}

func (m *mockDeployerForTest) GetEnvironmentInfo(ctx context.Context, spec *spec.EnclaveSpec) (*kurtosis.KurtosisEnvironment, error) {
	return &kurtosis.KurtosisEnvironment{}, nil
}

func TestDeploy(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a temporary directory for the environment output
	tmpDir, err := os.MkdirTemp("", "deploy-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a simple template file
	templatePath := filepath.Join(tmpDir, "template.yaml")
	err = os.WriteFile(templatePath, []byte("test: {{ .Config }}"), 0644)
	require.NoError(t, err)

	// Create a simple data file
	dataPath := filepath.Join(tmpDir, "data.json")
	err = os.WriteFile(dataPath, []byte(`{"Config": "value"}`), 0644)
	require.NoError(t, err)

	envPath := filepath.Join(tmpDir, "env.json")
	// Create a simple deployment configuration
	deployConfig := bytes.NewBufferString(`{"test": "config"}`)

	// Create a mock deployer function
	mockDeployerFunc := func(opts ...kurtosis.KurtosisDeployerOptions) (deployer, error) {
		return &mockDeployerForTest{baseDir: tmpDir}, nil
	}

	d := NewDeployer(
		WithBaseDir(tmpDir),
		WithKurtosisDeployer(mockDeployerFunc),
		WithDryRun(true),
		WithTemplateFile(templatePath),
		WithDataFile(dataPath),
	)

	env, err := d.Deploy(ctx, deployConfig)
	require.NoError(t, err)
	require.NotNil(t, env)

	// Verify the environment file was created
	assert.FileExists(t, envPath)

	// Read and verify the content
	content, err := os.ReadFile(envPath)
	require.NoError(t, err)

	var envData map[string]interface{}
	err = json.Unmarshal(content, &envData)
	require.NoError(t, err)
	assert.Equal(t, "value", envData["test"])
}
