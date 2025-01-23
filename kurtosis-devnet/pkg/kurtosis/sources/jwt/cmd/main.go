package main

import (
	"fmt"
	"os"

	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis/sources/jwt"
	"github.com/ethereum-optimism/optimism/op-service/cliapp"
	"github.com/urfave/cli/v2"
)

var (
	GitCommit = ""
	GitDate   = ""
)

func main() {
	app := cli.NewApp()
	app.Version = fmt.Sprintf("%s-%s", GitCommit, GitDate)
	app.Name = "jwt"
	app.Usage = "Tool to extract JWT secrets from Kurtosis enclaves"
	app.Flags = cliapp.ProtectFlags([]cli.Flag{
		&cli.StringFlag{
			Name:     "enclave",
			Usage:    "Name of the Kurtosis enclave",
			Required: true,
		},
	})
	app.Action = runJWT
	app.Writer = os.Stdout
	app.ErrWriter = os.Stderr

	err := app.Run(os.Args)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Application failed: %v\n", err)
		os.Exit(1)
	}
}

func runJWT(ctx *cli.Context) error {
	enclave := ctx.String("enclave")

	extractor := jwt.NewExtractor(enclave)
	data, err := extractor.ExtractData(ctx.Context)
	if err != nil {
		return fmt.Errorf("failed to extract JWT data: %w", err)
	}

	// Print the JWT secrets
	fmt.Printf("L1 JWT Secret: %s\n", data.L1JWT)
	fmt.Printf("L2 JWT Secret: %s\n", data.L2JWT)

	return nil
}
