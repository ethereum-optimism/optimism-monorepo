package main

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
	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/tmpl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"
)

type mockDeployer struct {
	dryRun bool
}

func (m *mockDeployer) Deploy(ctx context.Context, input io.Reader) (*spec.EnclaveSpec, error) {
	return &spec.EnclaveSpec{}, nil
}

func (m *mockDeployer) GetEnvironmentInfo(ctx context.Context, spec *spec.EnclaveSpec) (*kurtosis.KurtosisEnvironment, error) {
	return &kurtosis.KurtosisEnvironment{}, nil
}

func newMockDeployer(...kurtosis.KurtosisDeployerOptions) (deployer, error) {
	return &mockDeployer{dryRun: true}, nil
}

type mockEngineManager struct{}

func (m *mockEngineManager) EnsureRunning() error {
	return nil
}

func newTestMain(cfg *config) *Main {
	return &Main{
		cfg: cfg,
		newDeployer: func(opts ...kurtosis.KurtosisDeployerOptions) (deployer, error) {
			return newMockDeployer(opts...)
		},
		engineManager: &mockEngineManager{},
	}
}

func TestParseFlags(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantCfg   *config
		wantError bool
	}{
		{
			name: "valid configuration",
			args: []string{
				"--template", "path/to/template.yaml",
				"--enclave", "test-enclave",
			},
			wantCfg: &config{
				templateFile:    "path/to/template.yaml",
				enclave:         "test-enclave",
				kurtosisPackage: kurtosis.DefaultPackageName,
			},
			wantError: false,
		},
		{
			name:      "missing required template",
			args:      []string{"--enclave", "test-enclave"},
			wantCfg:   nil,
			wantError: true,
		},
		{
			name: "with data file",
			args: []string{
				"--template", "path/to/template.yaml",
				"--data", "path/to/data.json",
			},
			wantCfg: &config{
				templateFile:    "path/to/template.yaml",
				dataFile:        "path/to/data.json",
				enclave:         kurtosis.DefaultEnclave,
				kurtosisPackage: kurtosis.DefaultPackageName,
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cfg *config
			app := &cli.App{
				Flags: getFlags(),
				Action: func(c *cli.Context) (err error) {
					cfg, err = newConfig(c)
					return
				},
			}

			// Prepend program name to args as urfave/cli expects
			args := append([]string{"prog"}, tt.args...)

			err := app.Run(args)
			if tt.wantError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, cfg)
			assert.Equal(t, tt.wantCfg.templateFile, cfg.templateFile)
			assert.Equal(t, tt.wantCfg.enclave, cfg.enclave)
			assert.Equal(t, tt.wantCfg.kurtosisPackage, cfg.kurtosisPackage)
			if tt.wantCfg.dataFile != "" {
				assert.Equal(t, tt.wantCfg.dataFile, cfg.dataFile)
			}
		})
	}
}

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

	cfg := &config{
		templateFile: templatePath,
		dataFile:     dataPath,
		enclave:      "test-enclave",
		dryRun:       true, // Important for tests
	}

	m := newTestMain(cfg)

	buf, err := m.renderTemplate(tmpDir)
	require.NoError(t, err)

	// Verify template rendering
	assert.Contains(t, buf.String(), "test-deployment")
	assert.Contains(t, buf.String(), "test-project:test-enclave")
}

func TestDeploy(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a temporary directory for the environment output
	tmpDir, err := os.MkdirTemp("", "deploy-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	envPath := filepath.Join(tmpDir, "env.json")
	cfg := &config{
		environment: envPath,
		dryRun:      true,
	}

	// Create a simple deployment configuration
	deployConfig := bytes.NewBufferString(`{"test": "config"}`)

	m := newTestMain(cfg)
	err = m.deploy(ctx, deployConfig)
	require.NoError(t, err)

	// Verify the environment file was created
	assert.FileExists(t, envPath)

	// Read and verify the content
	content, err := os.ReadFile(envPath)
	require.NoError(t, err)

	var env map[string]interface{}
	err = json.Unmarshal(content, &env)
	require.NoError(t, err)
}

func TestDeployFileserver(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tmpDir, err := os.MkdirTemp("", "deploy-fileserver-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	envPath := filepath.Join(tmpDir, "env.json")
	cfg := &config{
		baseDir:     tmpDir,
		environment: envPath,
		dryRun:      true,
	}

	m := newTestMain(cfg)
	err = m.deployFileserver(ctx, filepath.Join(tmpDir, "fileserver"))
	require.NoError(t, err)
}

func TestMainFunc(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "main-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create test template
	templatePath := filepath.Join(tmpDir, "template.yaml")
	err = os.WriteFile(templatePath, []byte("name: test"), 0644)
	require.NoError(t, err)

	// Create environment output path
	envPath := filepath.Join(tmpDir, "env.json")

	cfg := &config{
		templateFile: templatePath,
		environment:  envPath,
		dryRun:       true,
	}

	m := newTestMain(cfg)
	err = m.run()
	require.NoError(t, err)

	// Verify the environment file was created
	assert.FileExists(t, envPath)
}

func TestLocalPrestate(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "prestate-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a mock justfile
	err = os.WriteFile(filepath.Join(tmpDir, "justfile"), []byte(`
_prestate-build target:
	@echo "Mock prestate build"
`), 0644)
	require.NoError(t, err)

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
			cfg := &config{
				baseDir: tmpDir,
				dryRun:  tt.dryRun,
			}

			m := newTestMain(cfg)

			tmpDir, err := os.MkdirTemp("", "prestate-test")
			require.NoError(t, err)
			defer os.RemoveAll(tmpDir)

			// Create template context with just the prestate function
			tmplCtx := tmpl.NewTemplateContext(m.localPrestateOption(tmpDir))

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
