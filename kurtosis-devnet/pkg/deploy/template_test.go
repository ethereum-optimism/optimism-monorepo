package deploy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderTemplate(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "template-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a test template file
	templateContent := `
name: {{.name}}
image: {{localDockerImage "test-project"}}
artifacts: {{localContractArtifacts "l1"}}`

	templatePath := filepath.Join(tmpDir, "template.yaml")
	err = os.WriteFile(templatePath, []byte(templateContent), 0644)
	require.NoError(t, err)

	// Create a test data file
	dataContent := `{"name": "test-deployment"}`
	dataPath := filepath.Join(tmpDir, "data.json")
	err = os.WriteFile(dataPath, []byte(dataContent), 0644)
	require.NoError(t, err)

	// Create a Templater instance
	templater := &Templater{
		enclave:      "test-enclave",
		dryRun:       true,
		baseDir:      tmpDir,
		templateFile: templatePath,
		dataFile:     dataPath,
		buildDir:     tmpDir,
		urlBuilder: func(path ...string) string {
			return "http://localhost:8080/" + strings.Join(path, "/")
		},
	}

	buf, err := templater.Render()
	require.NoError(t, err)

	// Verify template rendering
	assert.Contains(t, buf.String(), "test-deployment")
	assert.Contains(t, buf.String(), "test-project:test-enclave")
}
