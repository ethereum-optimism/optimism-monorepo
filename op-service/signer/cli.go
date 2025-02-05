package signer

import (
	"errors"
	"net/http"
	"strings"

	"github.com/urfave/cli/v2"

	opservice "github.com/ethereum-optimism/optimism/op-service"
	optls "github.com/ethereum-optimism/optimism/op-service/tls"
)

const (
	EndpointFlagName = "signer.endpoint"
	AddressFlagName  = "signer.address"
	HeadersFlagName  = "signer.header"
)

func CLIFlags(envPrefix string, category string) []cli.Flag {
	envPrefix += "_SIGNER"
	flags := []cli.Flag{
		&cli.StringFlag{
			Name:     EndpointFlagName,
			Usage:    "Signer endpoint the client will connect to",
			EnvVars:  opservice.PrefixEnvVar(envPrefix, "ENDPOINT"),
			Category: category,
		},
		&cli.StringFlag{
			Name:     AddressFlagName,
			Usage:    "Address the signer is signing requests for",
			EnvVars:  opservice.PrefixEnvVar(envPrefix, "ADDRESS"),
			Category: category,
		},
		&cli.StringSliceFlag{
			Name:    HeadersFlagName,
			Usage:   "Headers to pass to the remote signer. Format `key=value`. Value can contain any character allowed in a HTTP header. When using env vars, split with commas. When using flags one key value pair per flag.",
			EnvVars: opservice.PrefixEnvVar(envPrefix, "HEADER"),
		},
	}
	flags = append(flags, optls.CLIFlagsWithFlagPrefix(envPrefix, "signer", category)...)
	return flags
}

type CLIConfig struct {
	Endpoint  string
	Address   string
	Headers   http.Header
	TLSConfig optls.CLIConfig
}

func NewCLIConfig() CLIConfig {
	return CLIConfig{
		Headers:   http.Header{},
		TLSConfig: optls.NewCLIConfig(),
	}
}

func (c CLIConfig) Check() error {
	if err := c.TLSConfig.Check(); err != nil {
		return err
	}
	if !((c.Endpoint == "" && c.Address == "") || (c.Endpoint != "" && c.Address != "")) {
		return errors.New("signer endpoint and address must both be set or not set")
	}
	return nil
}

func (c CLIConfig) Enabled() bool {
	return c.Endpoint != "" && c.Address != ""
}

func ReadCLIConfig(ctx *cli.Context) CLIConfig {
	var headers = http.Header{}
	if ctx.StringSlice(HeadersFlagName) != nil {
		for _, header := range ctx.StringSlice(HeadersFlagName) {
			args := strings.SplitN(header, "=", 2)
			if len(args) == 2 {
				headers.Set(args[0], args[1])
			}
		}
	}

	cfg := CLIConfig{
		Endpoint:  ctx.String(EndpointFlagName),
		Address:   ctx.String(AddressFlagName),
		Headers:   headers,
		TLSConfig: optls.ReadCLIConfigWithPrefix(ctx, "signer"),
	}
	return cfg
}
