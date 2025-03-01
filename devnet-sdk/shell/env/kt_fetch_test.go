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
type testFS struct {
	artifacts map[string]bool
}

func (m *testFS) GetArtifact(_ context.Context, name string) (*ktfs.Artifact, error) {
	if name == "error" {
		return nil, fmt.Errorf("mock error")
	}
	if !m.artifacts[name] {
		return nil, fmt.Errorf("artifact %s not found", name)
	}
	// We don't need to return a real artifact since we're only testing error cases
	return nil, nil
}

func (m *testFS) GetAllArtifactNames(_ context.Context) ([]string, error) {
	if m.artifacts == nil {
		return nil, nil
	}
	names := make([]string, 0, len(m.artifacts))
	for name := range m.artifacts {
		names = append(names, name)
	}
	return names, nil
}

func (m *testFS) Close() error {
	return nil
}

var _ EnclaveFS = (*testFS)(nil)

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
			wantArtifact: "",
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
					return &testFS{artifacts: map[string]bool{"error": true}}, nil
				}
			},
			urlStr: "kt://myenclave/error",
		},
		{
			name: "no default descriptor",
			setupMock: func() {
				NewEnclaveFS = func(_ context.Context, _ string) (EnclaveFS, error) {
					return &testFS{artifacts: map[string]bool{}}, nil
				}
			},
			urlStr: "kt://myenclave",
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

func TestGetDefaultDescriptor(t *testing.T) {
	tests := []struct {
		name        string
		artifacts   map[string]bool
		wantName    string
		wantErrText string
	}{
		{
			name: "finds highest numbered descriptor",
			artifacts: map[string]bool{
				"devnet-descriptor-1":  true,
				"devnet-descriptor-5":  true,
				"devnet-descriptor-10": true,
				"other":                true,
			},
			wantName: "devnet-descriptor-10",
		},
		{
			name: "handles non-numeric suffixes",
			artifacts: map[string]bool{
				"devnet-descriptor-1":   true,
				"devnet-descriptor-5":   true,
				"devnet-descriptor-abc": true,
				"devnet-descriptor-10":  true,
				"other":                 true,
				"devnet-descriptor-def": true,
			},
			wantName: "devnet-descriptor-10",
		},
		{
			name:        "no descriptors",
			artifacts:   map[string]bool{},
			wantErrText: "no descriptor found with valid numerical suffix",
		},
		{
			name: "no valid descriptors",
			artifacts: map[string]bool{
				"other":      true,
				"devnet-abc": true,
			},
			wantErrText: "no descriptor found with valid numerical suffix",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := &testFS{artifacts: tt.artifacts}
			got, err := getDefaultDescriptor(context.Background(), fs)
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
