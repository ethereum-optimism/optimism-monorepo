package deploy

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	ktfs "github.com/ethereum-optimism/optimism/devnet-sdk/kt/fs"
	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis"
	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis/sources/spec"
	"github.com/kurtosis-tech/kurtosis/api/golang/core/kurtosis_core_rpc_api_bindings"
	"github.com/kurtosis-tech/kurtosis/api/golang/core/lib/services"
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

// mockEnclaveContext implements EnclaveContextIface for testing
type mockEnclaveContext struct {
	artifacts []string
}

func (m *mockEnclaveContext) GetAllFilesArtifactNamesAndUuids(ctx context.Context) ([]*kurtosis_core_rpc_api_bindings.FilesArtifactNameAndUuid, error) {
	result := make([]*kurtosis_core_rpc_api_bindings.FilesArtifactNameAndUuid, len(m.artifacts))
	for i, name := range m.artifacts {
		result[i] = &kurtosis_core_rpc_api_bindings.FilesArtifactNameAndUuid{
			FileName: name,
			FileUuid: "test-uuid",
		}
	}
	return result, nil
}

func (m *mockEnclaveContext) DownloadFilesArtifact(ctx context.Context, name string) ([]byte, error) {
	return nil, nil
}

func (m *mockEnclaveContext) UploadFiles(pathToUpload string, artifactName string) (services.FilesArtifactUUID, services.FileArtifactName, error) {
	return "", "", nil
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

	// Create a mock EnclaveFS function
	mockEnclaveFSFunc := func(ctx context.Context, enclave string) (*ktfs.EnclaveFS, error) {
		mockCtx := &mockEnclaveContext{
			artifacts: []string{
				"devnet-descriptor-1",
				"devnet-descriptor-2",
			},
		}
		return ktfs.NewEnclaveFSWithContext(mockCtx), nil
	}

	d := NewDeployer(
		WithBaseDir(tmpDir),
		WithKurtosisDeployer(mockDeployerFunc),
		WithDryRun(true),
		WithTemplateFile(templatePath),
		WithDataFile(dataPath),
		WithNewEnclaveFSFunc(mockEnclaveFSFunc),
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

func TestGetNextDevnetDescriptor(t *testing.T) {
	tests := []struct {
		name        string
		artifacts   []string
		wantName    string
		wantErrText string
	}{
		{
			name: "increments highest numbered descriptor",
			artifacts: []string{
				"devnet-descriptor-1",
				"devnet-descriptor-5",
				"devnet-descriptor-10",
				"other",
			},
			wantName: "devnet-descriptor-11",
		},
		{
			name: "handles non-numeric suffixes",
			artifacts: []string{
				"devnet-descriptor-1",
				"devnet-descriptor-5",
				"devnet-descriptor-abc",
				"devnet-descriptor-10",
				"other",
				"devnet-descriptor-def",
			},
			wantName: "devnet-descriptor-11",
		},
		{
			name:      "no descriptors",
			artifacts: []string{},
			wantName:  "devnet-descriptor-0",
		},
		{
			name: "no valid descriptors",
			artifacts: []string{
				"other",
				"devnet-abc",
			},
			wantName: "devnet-descriptor-0",
		},
		{
			name: "handles negative numbers",
			artifacts: []string{
				"devnet-descriptor--1",
				"devnet-descriptor-5",
			},
			wantName: "devnet-descriptor-6",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCtx := &mockEnclaveContext{artifacts: tt.artifacts}
			fs := ktfs.NewEnclaveFSWithContext(mockCtx)
			got, err := getNextDevnetDescriptor(context.Background(), fs)
			if tt.wantErrText != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrText)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantName, got)
		})
	}
}
