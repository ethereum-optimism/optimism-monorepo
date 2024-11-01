package tls

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
)

func TestDefaultCLIOptionsMatchDefaultConfig(t *testing.T) {
	cfg := configForArgs()
	defaultCfg := NewCLIConfig()
	require.Equal(t, defaultCfg, cfg)
}

func TestDefaultConfigIsValid(t *testing.T) {
	err := NewCLIConfig().Check()
	require.NoError(t, err)
}

func TestInvalidConfig(t *testing.T) {
	tests := []struct {
		name         string
		configChange func(config *CLIConfig)
	}{
		{"MissingCaCert", func(config *CLIConfig) {
			config.TLSCaCert = ""
		}},
		{"MissingCert", func(config *CLIConfig) {
			config.TLSCert = ""
		}},
		{"MissingKey", func(config *CLIConfig) {
			config.TLSKey = ""
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := NewCLIConfig()
			cfg.Enabled = true
			test.configChange(&cfg)
			err := cfg.Check()
			require.ErrorContains(t, err, "all tls flags must be set if at least one is set")
		})
		t.Run(fmt.Sprintf("%sAllowedWhenDisabled", test.name), func(t *testing.T) {
			cfg := NewCLIConfig()
			cfg.Enabled = false
			test.configChange(&cfg)
			require.NoError(t, cfg.Check())
		})
	}
}

func configForArgs(args ...string) CLIConfig {
	app := cli.NewApp()
	app.Flags = CLIFlagsWithFlagPrefix("TEST_", "test")
	app.Name = "test"
	var config CLIConfig
	app.Action = func(ctx *cli.Context) error {
		config = ReadCLIConfigWithPrefix(ctx, "test")
		return nil
	}
	_ = app.Run(args)
	return config
}
