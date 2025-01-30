package main

import (
	"fmt"
	"os"

	"github.com/ethereum-optimism/optimism/devnet-sdk/kt"
	"github.com/ethereum-optimism/optimism/devnet-sdk/manifest"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"
)

func main() {
	app := &cli.App{
		Name:  "devnet",
		Usage: "Generate Kurtosis parameters from a devnet manifest",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "manifest",
				Aliases:  []string{"m"},
				Usage:    "Path to the manifest YAML file",
				Required: true,
			},
			&cli.StringFlag{
				Name:    "output",
				Aliases: []string{"o"},
				Usage:   "Path to write the Kurtosis parameters file (default: stdout)",
			},
		},
		Action: func(c *cli.Context) error {
			// Read manifest file
			manifestPath := c.String("manifest")
			manifestBytes, err := os.ReadFile(manifestPath)
			if err != nil {
				return fmt.Errorf("failed to read manifest file: %w", err)
			}

			// Parse manifest YAML
			var m manifest.Manifest
			if err := yaml.Unmarshal(manifestBytes, &m); err != nil {
				return fmt.Errorf("failed to parse manifest YAML: %w", err)
			}

			// Create visitor and process manifest
			visitor := kt.NewKurtosisVisitor()
			m.Accept(visitor)

			// Get params and write to file or stdout
			params := visitor.GetParams()
			paramsBytes, err := yaml.Marshal(params)
			if err != nil {
				return fmt.Errorf("failed to marshal params: %w", err)
			}

			outputPath := c.String("output")
			if outputPath != "" {
				if err := os.WriteFile(outputPath, paramsBytes, 0644); err != nil {
					return fmt.Errorf("failed to write params file: %w", err)
				}
			} else {
				fmt.Print(string(paramsBytes))
			}

			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
