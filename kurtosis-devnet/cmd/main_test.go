package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
)

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

func TestMainFuncValidatesConfig(t *testing.T) {
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

	app := &cli.App{
		Flags: getFlags(),
		Action: func(c *cli.Context) error {
			cfg, err := newConfig(c)
			if err != nil {
				return err
			}

			// Verify config values
			assert.Equal(t, templatePath, cfg.templateFile)
			assert.Equal(t, envPath, cfg.environment)
			assert.True(t, cfg.dryRun)

			// Create an empty environment file to simulate successful deployment
			return os.WriteFile(envPath, []byte("{}"), 0644)
		},
	}

	args := []string{
		"prog",
		"--template", templatePath,
		"--environment", envPath,
		"--dry-run",
	}

	err = app.Run(args)
	require.NoError(t, err)

	// Verify the environment file was created
	assert.FileExists(t, envPath)
}
