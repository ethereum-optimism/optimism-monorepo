package env

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseKurtosisNativeURL(t *testing.T) {
	tests := []struct {
		name           string
		urlStr         string
		wantEnclave    string
		wantArtifact   string
		wantFile       string
		wantParseError bool
	}{
		{
			name:        "absolute file path",
			urlStr:      "ktnative://myenclave/path/args.yaml",
			wantEnclave: "myenclave",
			wantFile:    "/path/args.yaml",
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

			enclave, argsFile := parseKurtosisNativeURL(u)
			assert.Equal(t, tt.wantEnclave, enclave)
			assert.Equal(t, tt.wantFile, argsFile)
		})
	}
}
