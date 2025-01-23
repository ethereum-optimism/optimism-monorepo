package main

import (
	"fmt"
	"os"

	"github.com/ethereum-optimism/optimism/devnet-sdk/inventory"
	"github.com/ethereum-optimism/optimism/devnet-sdk/k8s"
	"github.com/ethereum-optimism/optimism/devnet-sdk/manifest"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"
)

func main() {
	app := &cli.App{
		Name:  "k8sgen",
		Usage: "Generate Kubernetes configuration from inventory",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "inventory",
				Aliases:  []string{"i"},
				Usage:    "Path to the inventory YAML file",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "manifest",
				Aliases:  []string{"m"},
				Usage:    "Path to the manifest YAML file",
				Required: true,
			},
			&cli.StringFlag{
				Name:    "output",
				Aliases: []string{"o"},
				Usage:   "Path to write the output file (default: stdout)",
			},
		},
		Action: func(c *cli.Context) error {
			// Read inventory file
			inventoryPath := c.String("inventory")
			inventoryBytes, err := os.ReadFile(inventoryPath)
			if err != nil {
				return fmt.Errorf("failed to read inventory file: %w", err)
			}

			// Parse inventory YAML
			var inv inventory.Inventory
			if err := yaml.Unmarshal(inventoryBytes, &inv); err != nil {
				return fmt.Errorf("failed to parse inventory YAML: %w", err)
			}

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

			// Create visitor and process inventory
			k8sVisitor := k8s.NewK8sVisitor()
			inv.Accept(k8sVisitor)
			m.Accept(k8sVisitor)
			k8sVisitor.BuildParams()

			k8sParams := k8sVisitor.GetParams()

			// Get nodes and write to file or stdout
			k8sParamsBytes, err := yaml.Marshal(k8sParams)
			if err != nil {
				return fmt.Errorf("failed to marshal nodes: %w", err)
			}

			outputPath := c.String("output")
			if outputPath != "" {
				if err := os.WriteFile(outputPath, k8sParamsBytes, 0644); err != nil {
					return fmt.Errorf("failed to write output file: %w", err)
				}
			} else {
				fmt.Print(string(k8sParamsBytes))
			}

			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
