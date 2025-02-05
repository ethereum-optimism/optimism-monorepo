package deploy

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/tmpl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestLocalPrestate(t *testing.T) {
	tests := []struct {
		name    string
		dryRun  bool
		wantErr bool
	}{
		{
			name:    "dry run mode",
			dryRun:  true,
			wantErr: false,
		},
		{
			name:    "normal mode",
			dryRun:  false,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("", "prestate-test")
			require.NoError(t, err)
			defer os.RemoveAll(tmpDir)

			// Create a mock justfile for each test case
			err = os.WriteFile(filepath.Join(tmpDir, "justfile"), []byte(`
_prestate-build target:
	@echo "Mock prestate build"
`), 0644)
			require.NoError(t, err)

			templater := &Templater{
				baseDir:  tmpDir,
				dryRun:   tt.dryRun,
				buildDir: tmpDir,
				urlBuilder: func(path ...string) string {
					return "http://fileserver/" + strings.Join(path, "/")
				},
			}

			// Create template context with just the prestate function
			tmplCtx := tmpl.NewTemplateContext(templater.localPrestateOption())

			// Test template with multiple calls to localPrestate
			template := `first:
  url: {{(localPrestate).URL}}
  hashes:
    game: {{index (localPrestate).Hashes "game"}}
    proof: {{index (localPrestate).Hashes "proof"}}
second:
  url: {{(localPrestate).URL}}
  hashes:
    game: {{index (localPrestate).Hashes "game"}}
    proof: {{index (localPrestate).Hashes "proof"}}`
			buf := bytes.NewBuffer(nil)
			err = tmplCtx.InstantiateTemplate(bytes.NewBufferString(template), buf)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)

			// Verify the output is valid YAML and contains the static path
			output := buf.String()
			assert.Contains(t, output, "url: http://fileserver/proofs/op-program/cannon")

			// Verify both calls return the same values
			var result struct {
				First struct {
					URL    string            `yaml:"url"`
					Hashes map[string]string `yaml:"hashes"`
				} `yaml:"first"`
				Second struct {
					URL    string            `yaml:"url"`
					Hashes map[string]string `yaml:"hashes"`
				} `yaml:"second"`
			}
			err = yaml.Unmarshal(buf.Bytes(), &result)
			require.NoError(t, err)

			// Check that both calls returned identical results
			assert.Equal(t, result.First.URL, result.Second.URL, "URLs should match")
			assert.Equal(t, result.First.Hashes, result.Second.Hashes, "Hashes should match")

			// Verify the directory was created only once
			prestateDir := filepath.Join(tmpDir, "proofs", "op-program", "cannon")
			assert.DirExists(t, prestateDir)
		})
	}
}
