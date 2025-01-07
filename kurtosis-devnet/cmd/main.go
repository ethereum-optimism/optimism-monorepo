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
	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis/api/engine"
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
		localHostName:   c.String("local-hostname"),
		kurtosisBinary:  c.String("kurtosis-binary"),
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

type engineManager interface {
	EnsureRunning() error
}

type Main struct {
	cfg           *config
	newDeployer   func(opts ...kurtosis.KurtosisDeployerOptions) (deployer, error)
	engineManager engineManager
}

func (m *Main) launchStaticServer(ctx context.Context) (*staticServer, func(), error) {
	// we will serve content from this tmpDir for the duration of the devnet creation
	tmpDir, err := os.MkdirTemp("", m.cfg.enclave)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating temporary directory: %w", err)
	}

	server := serve.NewServer(
		serve.WithStaticDir(tmpDir),
		serve.WithHostname(m.cfg.localHostName),
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

func (m *Main) localDockerImageOption() tmpl.TemplateContextOptions {
	dockerBuilder := build.NewDockerBuilder(
		build.WithDockerBaseDir(m.cfg.baseDir),
		build.WithDockerDryRun(m.cfg.dryRun),
	)

	imageTag := func(projectName string) string {
		return fmt.Sprintf("%s:%s", projectName, m.cfg.enclave)
	}

	return tmpl.WithFunction("localDockerImage", func(projectName string) (string, error) {
		return dockerBuilder.Build(projectName, imageTag(projectName))
	})
}

func (m *Main) localContractArtifactsOption(server *staticServer) tmpl.TemplateContextOptions {
	contractsBundle := fmt.Sprintf("contracts-bundle-%s.tar.gz", m.cfg.enclave)
	contractsBundlePath := func(_ string) string {
		return filepath.Join(server.dir, contractsBundle)
	}

	contractBuilder := build.NewContractBuilder(
		build.WithContractBaseDir(m.cfg.baseDir),
		build.WithContractDryRun(m.cfg.dryRun),
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

func (m *Main) localPrestateOption(server *staticServer) tmpl.TemplateContextOptions {
	prestateBuilder := build.NewPrestateBuilder(
		build.WithPrestateBaseDir(m.cfg.baseDir),
		build.WithPrestateDryRun(m.cfg.dryRun),
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

		if m.cfg.dryRun {
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

func (m *Main) renderTemplate(server *staticServer) (*bytes.Buffer, error) {
	opts := []tmpl.TemplateContextOptions{
		m.localDockerImageOption(),
		m.localContractArtifactsOption(server),
		m.localPrestateOption(server),
	}

	// Read and parse the data file if provided
	if m.cfg.dataFile != "" {
		data, err := os.ReadFile(m.cfg.dataFile)
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
	tmplFile, err := os.Open(m.cfg.templateFile)
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

func (m *Main) deploy(ctx context.Context, r io.Reader) error {
	// Create a multi reader to output deployment input to stdout
	buf := bytes.NewBuffer(nil)
	tee := io.TeeReader(r, buf)

	// Log the deployment input
	log.Println("Deployment input:")
	if _, err := io.Copy(os.Stdout, tee); err != nil {
		return fmt.Errorf("error copying deployment input: %w", err)
	}

	opts := []kurtosis.KurtosisDeployerOptions{
		kurtosis.WithKurtosisBaseDir(m.cfg.baseDir),
		kurtosis.WithKurtosisDryRun(m.cfg.dryRun),
		kurtosis.WithKurtosisPackageName(m.cfg.kurtosisPackage),
		kurtosis.WithKurtosisEnclave(m.cfg.enclave),
	}

	d, err := m.newDeployer(opts...)
	if err != nil {
		return fmt.Errorf("error creating kurtosis deployer: %w", err)
	}

	env, err := d.Deploy(ctx, buf)
	if err != nil {
		return fmt.Errorf("error deploying kurtosis package: %w", err)
	}

	if err := writeEnvironment(m.cfg.environment, env); err != nil {
		return fmt.Errorf("error writing environment: %w", err)
	}

	return nil
}

type deployer interface {
	Deploy(ctx context.Context, input io.Reader) (*kurtosis.KurtosisEnvironment, error)
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

func (m *Main) run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if !m.cfg.dryRun {
		if err := m.engineManager.EnsureRunning(); err != nil {
			return fmt.Errorf("error ensuring kurtosis engine is running: %w", err)
		}
	}

	server, cleanup, err := m.launchStaticServer(ctx)
	if err != nil {
		return fmt.Errorf("error launching static server: %w", err)
	}
	defer cleanup()

	buf, err := m.renderTemplate(server)
	if err != nil {
		return fmt.Errorf("error rendering template: %w", err)
	}

	return m.deploy(ctx, buf)
}

func mainAction(c *cli.Context) error {
	cfg, err := newConfig(c)
	if err != nil {
		return fmt.Errorf("error parsing config: %w", err)
	}
	m := &Main{
		cfg: cfg,
		newDeployer: func(opts ...kurtosis.KurtosisDeployerOptions) (deployer, error) {
			return kurtosis.NewKurtosisDeployer(opts...)
		},
		engineManager: engine.NewEngineManager(engine.WithKurtosisBinary(cfg.kurtosisBinary)),
	}
	return m.run()
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
