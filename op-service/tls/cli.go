// This file contains CLI and env TLS configurations that can be used by clients or servers
package tls

import (
	"errors"
	"fmt"
	"strings"

	"github.com/urfave/cli/v2"

	opservice "github.com/ethereum-optimism/optimism/op-service"
)

const (
	TLSCaCertFlagName  = "tls.ca"
	TLSCertFlagName    = "tls.cert"
	TLSKeyFlagName     = "tls.key"
	TLSEnabledFlagName = "tls.enabled"
)

// CLIFlags returns flags with env var envPrefix
// This should be used for server TLS configs, or when client and server tls configs are the same
func CLIFlags(envPrefix string) []cli.Flag {
	return CLIFlagsWithFlagPrefix(envPrefix, "")
}

var (
	defaultTLSCaCert  = "tls/ca.crt"
	defaultTLSCert    = "tls/tls.crt"
	defaultTLSKey     = "tls/tls.key"
	defaultTLSEnabled = true
)

// CLIFlagsWithFlagPrefix returns flags with env var and cli flag prefixes
// Should be used for client TLS configs when different from server on the same process
func CLIFlagsWithFlagPrefix(envPrefix string, flagPrefix string) []cli.Flag {
	prefixFunc := func(flagName string) string {
		return strings.Trim(fmt.Sprintf("%s.%s", flagPrefix, flagName), ".")
	}
	prefixEnvVars := func(name string) []string {
		return opservice.PrefixEnvVar(envPrefix, name)
	}
	return []cli.Flag{
		&cli.BoolFlag{
			Name:    prefixFunc(TLSEnabledFlagName),
			Usage:   "Enable or disable TLS client authentication for the signer",
			Value:   defaultTLSEnabled,
			EnvVars: prefixEnvVars("TLS_ENABLED"),
		},
		&cli.StringFlag{
			Name:    prefixFunc(TLSCaCertFlagName),
			Usage:   "tls ca cert path",
			Value:   defaultTLSCaCert,
			EnvVars: prefixEnvVars("TLS_CA"),
		},
		&cli.StringFlag{
			Name:    prefixFunc(TLSCertFlagName),
			Usage:   "tls cert path",
			Value:   defaultTLSCert,
			EnvVars: prefixEnvVars("TLS_CERT"),
		},
		&cli.StringFlag{
			Name:    prefixFunc(TLSKeyFlagName),
			Usage:   "tls key",
			Value:   defaultTLSKey,
			EnvVars: prefixEnvVars("TLS_KEY"),
		},
	}
}

type CLIConfig struct {
	TLSCaCert string
	TLSCert   string
	TLSKey    string
	Enabled   bool
}

func NewCLIConfig() CLIConfig {
	return CLIConfig{
		TLSCaCert: defaultTLSCaCert,
		TLSCert:   defaultTLSCert,
		TLSKey:    defaultTLSKey,
		Enabled:   true,
	}
}

func (c CLIConfig) Check() error {
	if c.TLSEnabled() && (c.TLSCaCert == "" || c.TLSCert == "" || c.TLSKey == "") {
		return errors.New("all tls flags must be set if at least one is set")
	}

	return nil
}

func (c CLIConfig) TLSEnabled() bool {
	return c.Enabled
}

// ReadCLIConfig reads tls cli configs
// This should be used for server TLS configs, or when client and server tls configs are the same
func ReadCLIConfig(ctx *cli.Context) CLIConfig {
	return CLIConfig{
		TLSCaCert: ctx.String(TLSCaCertFlagName),
		TLSCert:   ctx.String(TLSCertFlagName),
		TLSKey:    ctx.String(TLSKeyFlagName),
		Enabled:   ctx.Bool(TLSEnabledFlagName),
	}
}

// ReadCLIConfigWithPrefix reads tls cli configs with flag prefix
// Should be used for client TLS configs when different from server on the same process
func ReadCLIConfigWithPrefix(ctx *cli.Context, flagPrefix string) CLIConfig {
	prefixFunc := func(flagName string) string {
		return strings.Trim(fmt.Sprintf("%s.%s", flagPrefix, flagName), ".")
	}
	return CLIConfig{
		TLSCaCert: ctx.String(prefixFunc(TLSCaCertFlagName)),
		TLSCert:   ctx.String(prefixFunc(TLSCertFlagName)),
		TLSKey:    ctx.String(prefixFunc(TLSKeyFlagName)),
		Enabled:   ctx.Bool(prefixFunc(TLSEnabledFlagName)),
	}
}
