package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/deploy"
	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis"
	"github.com/urfave/cli/v2"
)

type config struct {
	templateFile    string
	dataFile        string
	kurtosisPackage string
	enclave         string
	environment     string
	dryRun          bool
	baseDir         string
	kurtosisBinary  string
}

func newConfig(c *cli.Context) (*config, error) {
	cfg := &config{
		templateFile:    c.String("template"),
		dataFile:        c.String("data"),
		kurtosisPackage: c.String("kurtosis-package"),
		enclave:         c.String("enclave"),
		environment:     c.String("environment"),
		dryRun:          c.Bool("dry-run"),
		kurtosisBinary:  c.String("kurtosis-binary"),
	}

	// Validate required flags
	if cfg.templateFile == "" {
		return nil, fmt.Errorf("template file is required")
	}
	cfg.baseDir = filepath.Dir(cfg.templateFile)

	return cfg, nil
}

func writeEnvironment(path string, env *kurtosis.KurtosisEnvironment) error {
	out := os.Stdout
	if path != "" {
		var err error
		out, err = os.Create(path)
		if err != nil {
			return fmt.Errorf("error creating environment file: %w", err)
		}
		defer out.Close()
	}

	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	if err := enc.Encode(env); err != nil {
		return fmt.Errorf("error encoding environment: %w", err)
	}

	return nil
}

func mainAction(c *cli.Context) error {
	ctx := context.Background()

	cfg, err := newConfig(c)
	if err != nil {
		return fmt.Errorf("error parsing config: %w", err)
	}

	deployer := deploy.NewDeployer(
		deploy.WithKurtosisPackage(cfg.kurtosisPackage),
		deploy.WithEnclave(cfg.enclave),
		deploy.WithDryRun(cfg.dryRun),
		deploy.WithKurtosisBinary(cfg.kurtosisBinary),
		deploy.WithTemplateFile(cfg.templateFile),
		deploy.WithDataFile(cfg.dataFile),
		deploy.WithBaseDir(cfg.baseDir),
	)

	env, err := deployer.Deploy(ctx, nil)
	if err != nil {
		return fmt.Errorf("error deploying environment: %w", err)
	}

	return writeEnvironment(cfg.environment, env)
}

func getFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:     "template",
			Usage:    "Path to the template file (required)",
			Required: true,
		},
		&cli.StringFlag{
			Name:  "data",
			Usage: "Path to JSON data file (optional)",
		},
		&cli.StringFlag{
			Name:  "kurtosis-package",
			Usage: "Kurtosis package to deploy (optional)",
			Value: kurtosis.DefaultPackageName,
		},
		&cli.StringFlag{
			Name:  "enclave",
			Usage: "Enclave name (optional)",
			Value: kurtosis.DefaultEnclave,
		},
		&cli.StringFlag{
			Name:  "environment",
			Usage: "Path to JSON environment file output (optional)",
		},
		&cli.BoolFlag{
			Name:  "dry-run",
			Usage: "Dry run mode (optional)",
		},
		&cli.StringFlag{
			Name:  "kurtosis-binary",
			Usage: "Path to kurtosis binary (optional)",
			Value: "kurtosis",
		},
	}
}

func main() {
	app := &cli.App{
		Name:   "kurtosis-devnet",
		Usage:  "Deploy and manage Optimism devnet using Kurtosis",
		Flags:  getFlags(),
		Action: mainAction,
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatalf("Error: %v\n", err)
	}
}
