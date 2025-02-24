package env

import (
	"context"
	"fmt"
	"net/url"
	"testing"

	ktfs "github.com/ethereum-optimism/optimism/devnet-sdk/kt/fs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testFS implements EnclaveFS for testing
type testFS struct{}

func (m *testFS) GetArtifact(_ context.Context, name string) (*ktfs.Artifact, error) {
	if name == "error" {
		return nil, fmt.Errorf("mock error")
	}
	// We don't need to return a real artifact since we're only testing error cases
	return nil, nil
}

func (m *testFS) Close() error {
	return nil
}

func TestParseKurtosisURL(t *testing.T) {
	tests := []struct {
		name           string
		urlStr         string
		wantEnclave    string
		wantArtifact   string
		wantFile       string
		wantParseError bool
	}{
		{
			name:         "basic url",
			urlStr:       "kt://myenclave",
			wantEnclave:  "myenclave",
			wantArtifact: "devnet",
			wantFile:     "env.json",
		},
		{
			name:         "with artifact",
			urlStr:       "kt://myenclave/custom-artifact",
			wantEnclave:  "myenclave",
			wantArtifact: "custom-artifact",
			wantFile:     "env.json",
		},
		{
			name:         "with artifact and file",
			urlStr:       "kt://myenclave/custom-artifact/config.json",
			wantEnclave:  "myenclave",
			wantArtifact: "custom-artifact",
			wantFile:     "config.json",
		},
		{
			name:         "with trailing slash",
			urlStr:       "kt://enclave/artifact/",
			wantEnclave:  "enclave",
			wantArtifact: "artifact",
			wantFile:     "env.json",
		},
		{
			name:           "invalid url",
			urlStr:         "://invalid",
			wantParseError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := url.Parse(tt.urlStr)
			if tt.wantParseError {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)

			enclave, artifact, file := parseKurtosisURL(u)
			assert.Equal(t, tt.wantEnclave, enclave)
			assert.Equal(t, tt.wantArtifact, artifact)
			assert.Equal(t, tt.wantFile, file)
		})
	}
}

func TestFetchKurtosisDataErrors(t *testing.T) {
	tests := []struct {
		name      string
		setupMock func()
		urlStr    string
	}{
		{
			name: "error creating fs",
			setupMock: func() {
				NewEnclaveFS = func(_ context.Context, _ string) (EnclaveFS, error) {
					return nil, fmt.Errorf("mock error")
				}
			},
			urlStr: "kt://myenclave",
		},
		{
			name: "error getting artifact",
			setupMock: func() {
				NewEnclaveFS = func(_ context.Context, _ string) (EnclaveFS, error) {
					return &testFS{}, nil
				}
			},
			urlStr: "kt://myenclave/error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origNewEnclaveFS := NewEnclaveFS
			defer func() { NewEnclaveFS = origNewEnclaveFS }()

			tt.setupMock()

			u, err := url.Parse(tt.urlStr)
			require.NoError(t, err)

			_, _, err = fetchKurtosisData(u)
			assert.Error(t, err)
		})
	}
}
