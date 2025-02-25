package env

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchFileData(t *testing.T) {
	// Create a temporary test file
	content := []byte("test content")
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.json")
	err := os.WriteFile(tmpFile, content, 0644)
	require.NoError(t, err)

	tests := []struct {
		name        string
		path        string
		wantName    string
		wantContent []byte
		wantError   bool
	}{
		{
			name:        "existing file",
			path:        tmpFile,
			wantName:    "test",
			wantContent: content,
		},
		{
			name:      "non-existent file",
			path:      filepath.Join(tmpDir, "nonexistent.json"),
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &url.URL{Path: tt.path}
			name, content, err := fetchFileData(u)
			if tt.wantError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantName, name)
			assert.Equal(t, tt.wantContent, content)
		})
	}
}

type mockOS struct {
	files map[string][]byte
}

func (m *mockOS) ReadFile(name string) ([]byte, error) {
	if content, ok := m.files[name]; ok {
		return content, nil
	}
	return nil, fmt.Errorf("file not found: %s", name)
}

func TestFetchFileDataFromOS(t *testing.T) {
	var (
		absoluteContent = []byte("absolute path content")
		relativeContent = []byte("relative path content")
	)

	mockOS := &mockOS{
		files: map[string][]byte{
			"/some/absolute/path": absoluteContent,
			"some/relative/path":  relativeContent,
		},
	}

	tests := []struct {
		name        string
		urlStr      string
		wantName    string
		wantContent []byte
		wantError   bool
	}{
		{
			name:        "file URL",
			urlStr:      "file:///some/absolute/path",
			wantName:    "path",
			wantContent: absoluteContent,
		},
		{
			name:        "absolute path",
			urlStr:      "/some/absolute/path",
			wantName:    "path",
			wantContent: absoluteContent,
		},
		{
			name:        "relative path",
			urlStr:      "some/relative/path",
			wantName:    "path",
			wantContent: relativeContent,
		},
		{
			name:      "non-existent file",
			urlStr:    "file:///nonexistent/path",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := url.Parse(tt.urlStr)
			require.NoError(t, err)

			name, content, err := fetchFileDataFromOS(u, mockOS)
			if tt.wantError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantName, name)
			assert.Equal(t, tt.wantContent, content)
		})
	}
}
