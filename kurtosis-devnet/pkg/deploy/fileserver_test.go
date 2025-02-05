package deploy

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis"
	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis/sources/spec"
	"github.com/stretchr/testify/require"
)

func TestDeployFileserver(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tmpDir, err := os.MkdirTemp("", "deploy-fileserver-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a mock deployer function
	mockDeployerFunc := func(opts ...kurtosis.KurtosisDeployerOptions) (deployer, error) {
		return &mockDeployer{}, nil
	}

	testCases := []struct {
		name        string
		fs          *FileServer
		shouldError bool
	}{
		{
			name: "successful deployment",
			fs: &FileServer{
				baseDir:  tmpDir,
				enclave:  "test-enclave",
				dryRun:   true,
				deployer: mockDeployerFunc,
			},
			shouldError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.fs.Deploy(ctx, filepath.Join(tmpDir, "fileserver"))
			if tc.shouldError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// mockDeployer implements the deployer interface for testing
type mockDeployer struct{}

func (m *mockDeployer) Deploy(ctx context.Context, input io.Reader) (*spec.EnclaveSpec, error) {
	return &spec.EnclaveSpec{}, nil
}

func (m *mockDeployer) GetEnvironmentInfo(ctx context.Context, spec *spec.EnclaveSpec) (*kurtosis.KurtosisEnvironment, error) {
	return &kurtosis.KurtosisEnvironment{}, nil
}
