package env

import (
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
