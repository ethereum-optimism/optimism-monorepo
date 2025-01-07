package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/build"
	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis"
	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis/backend"
	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/serve"
	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/tmpl"
	"github.com/urfave/cli/v2"
)

type config struct {
	templateFile    string
	dataFile        string
	kurtosisPackage string
	enclave         string
	environment     string
	dryRun          bool
	localHostName   string
	baseDir         string
}

func newConfig(c *cli.Context) (*config, error) {
	cfg := &config{
		templateFile:    c.String("template"),
		dataFile:        c.String("data"),
		kurtosisPackage: c.String("kurtosis-package"),
		enclave:         c.String("enclave"),
		environment:     c.String("environment"),
		dryRun:          c.Bool("dry-run"),
		localHostName:   c.String("local-hostname"),
	}

	// Validate required flags
	if cfg.templateFile == "" {
		return nil, fmt.Errorf("template file is required")
	}
	cfg.baseDir = filepath.Dir(cfg.templateFile)

	return cfg, nil
}

type staticServer struct {
	dir string
	*serve.Server
}

func launchStaticServer(ctx context.Context, cfg *config) (*staticServer, func(), error) {
	// we will serve content from this tmpDir for the duration of the devnet creation
	tmpDir, err := os.MkdirTemp("", cfg.enclave)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating temporary directory: %w", err)
	}

	server := serve.NewServer(
		serve.WithStaticDir(tmpDir),
		serve.WithHostname(cfg.localHostName),
	)
	if err := server.Start(ctx); err != nil {
		return nil, nil, fmt.Errorf("error starting server: %w", err)
	}

	return &staticServer{
			dir:    tmpDir,
			Server: server,
		}, func() {
			if err := server.Stop(ctx); err != nil {
				log.Printf("Error stopping server: %v\n", err)
			}
			if err := os.RemoveAll(tmpDir); err != nil {
				log.Printf("Error removing temporary directory: %v\n", err)
			}
		}, nil
}

func localDockerImageOption(cfg *config) tmpl.TemplateContextOptions {
	dockerBuilder := build.NewDockerBuilder(
		build.WithDockerBaseDir(cfg.baseDir),
		build.WithDockerDryRun(cfg.dryRun),
	)

	imageTag := func(projectName string) string {
		return fmt.Sprintf("%s:%s", projectName, cfg.enclave)
	}

	return tmpl.WithFunction("localDockerImage", func(projectName string) (string, error) {
		return dockerBuilder.Build(projectName, imageTag(projectName))
	})
}

func localContractArtifactsOption(cfg *config, server *staticServer) tmpl.TemplateContextOptions {
	contractsBundle := fmt.Sprintf("contracts-bundle-%s.tar.gz", cfg.enclave)
	contractsBundlePath := func(_ string) string {
		return filepath.Join(server.dir, contractsBundle)
	}

	contractBuilder := build.NewContractBuilder(
		build.WithContractBaseDir(cfg.baseDir),
		build.WithContractDryRun(cfg.dryRun),
	)

	return tmpl.WithFunction("localContractArtifacts", func(layer string) (string, error) {
		bundlePath := contractsBundlePath(layer)
		// we're in a temp dir, so we can skip the build if the file already
		// exists: it'll be the same file! In particular, since we're ignoring
		// layer for now, skip the 2nd build.
		if _, err := os.Stat(bundlePath); err != nil {
			if err := contractBuilder.Build(layer, bundlePath); err != nil {
				return "", err
			}
		}

		url := fmt.Sprintf("%s/%s", server.URL(), contractsBundle)
		log.Printf("%s: contract artifacts available at: %s\n", layer, url)
		return url, nil
	})
}

func localPrestateOption(cfg *config, server *staticServer) tmpl.TemplateContextOptions {
	prestateBuilder := build.NewPrestateBuilder(
		build.WithPrestateBaseDir(cfg.baseDir),
		build.WithPrestateDryRun(cfg.dryRun),
	)

	return tmpl.WithFunction("localPrestate", func() (string, error) {
		// Create build directory with the final path structure
		buildDir := filepath.Join(server.dir, "proofs", "op-program", "cannon")
		if err := os.MkdirAll(buildDir, 0755); err != nil {
			return "", fmt.Errorf("failed to create prestate build directory: %w", err)
		}

		// Get the relative path from server.dir to buildDir for the URL
		relPath, err := filepath.Rel(server.dir, buildDir)
		if err != nil {
			return "", fmt.Errorf("failed to get relative path: %w", err)
		}

		url := fmt.Sprintf("%s/%s", server.URL(), relPath)

		if cfg.dryRun {
			return url, nil
		}

		// Check if we already have prestate files. Typical in interop mode,
		// where we have a prestate for each chain.
		if dir, _ := os.ReadDir(buildDir); len(dir) > 0 {
			return url, nil
		}

		// Build all prestate files directly in the target directory
		if err := prestateBuilder.Build(buildDir); err != nil {
			return "", fmt.Errorf("failed to build prestates: %w", err)
		}

		// Find all prestate-proof*.json files
		matches, err := filepath.Glob(filepath.Join(buildDir, "prestate-proof*.json"))
		if err != nil {
			return "", fmt.Errorf("failed to find prestate files: %w", err)
		}

		// Process each file to rename it to its hash
		for _, filePath := range matches {
			content, err := os.ReadFile(filePath)
			if err != nil {
				return "", fmt.Errorf("failed to read prestate %s: %w", filepath.Base(filePath), err)
			}

			var data struct {
				Pre string `json:"pre"`
			}
			if err := json.Unmarshal(content, &data); err != nil {
				return "", fmt.Errorf("failed to parse prestate %s: %w", filepath.Base(filePath), err)
			}

			// Rename the file to just the hash
			hashedPath := filepath.Join(buildDir, data.Pre)
			if err := os.Rename(filePath, hashedPath); err != nil {
				return "", fmt.Errorf("failed to rename prestate %s: %w", filepath.Base(filePath), err)
			}

			log.Printf("%s available at: %s/%s/%s\n", filepath.Base(filePath), server.URL(), relPath, data.Pre)
		}

		return url, nil
	})
}

func renderTemplate(cfg *config, server *staticServer) (*bytes.Buffer, error) {
	opts := []tmpl.TemplateContextOptions{
		localDockerImageOption(cfg),
		localContractArtifactsOption(cfg, server),
		localPrestateOption(cfg, server),
	}

	// Read and parse the data file if provided
	if cfg.dataFile != "" {
		data, err := os.ReadFile(cfg.dataFile)
		if err != nil {
			return nil, fmt.Errorf("error reading data file: %w", err)
		}

		var templateData map[string]interface{}
		if err := json.Unmarshal(data, &templateData); err != nil {
			return nil, fmt.Errorf("error parsing JSON data: %w", err)
		}

		opts = append(opts, tmpl.WithData(templateData))
	}

	// Open template file
	tmplFile, err := os.Open(cfg.templateFile)
	if err != nil {
		return nil, fmt.Errorf("error opening template file: %w", err)
	}
	defer tmplFile.Close()

	// Create template context
	tmplCtx := tmpl.NewTemplateContext(opts...)

	// Process template
	buf := bytes.NewBuffer(nil)
	if err := tmplCtx.InstantiateTemplate(tmplFile, buf); err != nil {
		return nil, fmt.Errorf("error processing template: %w", err)
	}

	return buf, nil
}

func deploy(ctx context.Context, cfg *config, r io.Reader) error {
	// Create a multi reader to output deployment input to stdout
	buf := bytes.NewBuffer(nil)
	tee := io.TeeReader(r, buf)

	// Log the deployment input
	log.Println("Deployment input:")
	if _, err := io.Copy(os.Stdout, tee); err != nil {
		return fmt.Errorf("error copying deployment input: %w", err)
	}

	kurtosisDeployer, err := kurtosis.NewKurtosisDeployer(
		kurtosis.WithKurtosisBaseDir(cfg.baseDir),
		kurtosis.WithKurtosisDryRun(cfg.dryRun),
		kurtosis.WithKurtosisPackageName(cfg.kurtosisPackage),
		kurtosis.WithKurtosisEnclave(cfg.enclave),
	)
	if err != nil {
		return fmt.Errorf("error creating kurtosis deployer: %w", err)
	}

	env, err := kurtosisDeployer.Deploy(ctx, buf)
	if err != nil {
		return fmt.Errorf("error deploying kurtosis: %w", err)
	}

	envOutput := os.Stdout
	if cfg.environment != "" {
		envOutput, err = os.Create(cfg.environment)
		if err != nil {
			return fmt.Errorf("error creating environment file: %w", err)
		}
		defer envOutput.Close()
	} else {
		log.Println("\nEnvironment description:")
	}

	enc := json.NewEncoder(envOutput)
	enc.SetIndent("", "  ")
	if err := enc.Encode(env); err != nil {
		return fmt.Errorf("error encoding environment: %w", err)
	}

	return nil
}

func mainFunc(cfg *config) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server, cleanup, err := launchStaticServer(ctx, cfg)
	if err != nil {
		return fmt.Errorf("error launching static server: %w", err)
	}
	defer cleanup()

	buf, err := renderTemplate(cfg, server)
	if err != nil {
		return fmt.Errorf("error rendering template: %w", err)
	}

	return deploy(ctx, cfg, buf)
}

func mainAction(c *cli.Context) error {
	cfg, err := newConfig(c)
	if err != nil {
		return fmt.Errorf("error parsing config: %w", err)
	}
	return mainFunc(cfg)
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
			Name:  "local-hostname",
			Usage: "DNS for localhost from Kurtosis perspective (optional)",
			Value: backend.DefaultDockerHost(),
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
