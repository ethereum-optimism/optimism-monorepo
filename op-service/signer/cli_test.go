package signer

import (
	"net/http"
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

func TestHeaderParsing(t *testing.T) {
	testHeaders := []string{
		"test-key=this:is:a:value",
		"b64-test-key=value:dGVzdCBkYXRhIDE=$",
	}

	args := []string{"app", "--signer.header", testHeaders[0], "--signer.header", testHeaders[1]}
	cfg := configForArgs(args...)

	expectedHeaders := http.Header{}
	expectedHeaders.Set("test-key", "this:is:a:value")
	expectedHeaders.Set("b64-test-key", "value:dGVzdCBkYXRhIDE=$")

	require.Equal(t, expectedHeaders, cfg.Headers)
}

func TestHeaderParsingWithComma(t *testing.T) {
	testHeaders := []string{
		"test-key=this:is:a:value,b64-test-key=value:dGVzdCBkYXRhIDE=$",
	}

	args := []string{"app", "--signer.header", testHeaders[0]}
	cfg := configForArgs(args...)

	expectedHeaders := http.Header{}
	expectedHeaders.Set("test-key", "this:is:a:value")
	expectedHeaders.Set("b64-test-key", "value:dGVzdCBkYXRhIDE=$")

	require.Equal(t, expectedHeaders, cfg.Headers)
}

func TestInvalidConfig(t *testing.T) {
	tests := []struct {
		name         string
		expected     string
		configChange func(config *CLIConfig)
	}{
		{
			name:     "MissingEndpoint",
			expected: "signer endpoint and address must both be set or not set",
			configChange: func(config *CLIConfig) {
				config.Address = "0x1234"
				config.TLSConfig.Enabled = true
			},
		},
		{
			name:     "MissingAddress",
			expected: "signer endpoint and address must both be set or not set",
			configChange: func(config *CLIConfig) {
				config.Endpoint = "http://localhost"
				config.TLSConfig.Enabled = true
			},
		},
		{
			name:     "InvalidTLSConfig",
			expected: "all tls flags must be set if at least one is set",
			configChange: func(config *CLIConfig) {
				config.TLSConfig.TLSKey = ""
				config.TLSConfig.Enabled = true
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := NewCLIConfig()
			test.configChange(&cfg)
			err := cfg.Check()
			require.ErrorContains(t, err, test.expected)
		})
	}
}

func configForArgs(args ...string) CLIConfig {
	app := cli.NewApp()
	app.Flags = CLIFlags("TEST_", "")
	app.Name = "test"
	var config CLIConfig
	app.Action = func(ctx *cli.Context) error {
		config = ReadCLIConfig(ctx)
		return nil
	}
	_ = app.Run(args)
	return config
}
