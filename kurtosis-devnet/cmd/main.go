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
	"strings"

	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/build"
	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis"
	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis/api/engine"
	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis/sources/spec"
	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/tmpl"
	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/util"
	"github.com/urfave/cli/v2"
)

const FILESERVER_PACKAGE = "fileserver"

func fileserverURL(path ...string) string {
	return fmt.Sprintf("http://%s/%s", FILESERVER_PACKAGE, strings.Join(path, "/"))
}

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

type engineManager interface {
	EnsureRunning() error
}

type Main struct {
	cfg           *config
	newDeployer   func(opts ...kurtosis.KurtosisDeployerOptions) (deployer, error)
	engineManager engineManager
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

func (m *Main) localContractArtifactsOption(dir string) tmpl.TemplateContextOptions {
	contractsBundle := fmt.Sprintf("contracts-bundle-%s.tar.gz", m.cfg.enclave)
	contractsBundlePath := func(_ string) string {
		return filepath.Join(dir, contractsBundle)
	}
	contractsURL := fileserverURL(contractsBundle)

	contractBuilder := build.NewContractBuilder(
		build.WithContractBaseDir(m.cfg.baseDir),
		build.WithContractDryRun(m.cfg.dryRun),
	)

	return tmpl.WithFunction("localContractArtifacts", func(layer string) (string, error) {
		bundlePath := contractsBundlePath(layer)
		if err := contractBuilder.Build(layer, bundlePath); err != nil {
			return "", err
		}

		log.Printf("%s: contract artifacts available at: %s\n", layer, contractsURL)
		return contractsURL, nil
	})
}

type PrestateInfo struct {
	URL    string            `json:"url"`
	Hashes map[string]string `json:"hashes"`
}

type localPrestateHolder struct {
	info     *PrestateInfo
	baseDir  string
	buildDir string
	dryRun   bool
	builder  *build.PrestateBuilder
}

func newLocalPrestateHolder(baseDir string, buildDir string, dryRun bool) *localPrestateHolder {
	return &localPrestateHolder{
		baseDir:  baseDir,
		buildDir: buildDir,
		dryRun:   dryRun,
		builder: build.NewPrestateBuilder(
			build.WithPrestateBaseDir(baseDir),
			build.WithPrestateDryRun(dryRun),
		),
	}
}

func (h *localPrestateHolder) GetPrestateInfo() (*PrestateInfo, error) {
	if h.info != nil {
		return h.info, nil
	}

	prestatePath := []string{"proofs", "op-program", "cannon"}
	prestateURL := fileserverURL(prestatePath...)

	// Create build directory with the final path structure
	buildDir := filepath.Join(append([]string{h.buildDir}, prestatePath...)...)
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create prestate build directory: %w", err)
	}

	info := &PrestateInfo{
		URL:    prestateURL,
		Hashes: make(map[string]string),
	}

	if h.dryRun {
		h.info = info
		return info, nil
	}

	// Map of known file prefixes to their keys
	fileToKey := map[string]string{
		"prestate-proof.json":         "prestate",
		"prestate-proof-mt64.json":    "prestate_mt64",
		"prestate-proof-mt.json":      "prestate_mt",
		"prestate-proof-interop.json": "prestate_interop",
	}

	// Build all prestate files directly in the target directory
	if err := h.builder.Build(buildDir); err != nil {
		return nil, fmt.Errorf("failed to build prestates: %w", err)
	}

	// Find and process all prestate files
	matches, err := filepath.Glob(filepath.Join(buildDir, "prestate-proof*.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to find prestate files: %w", err)
	}

	// Process each file to rename it to its hash
	for _, filePath := range matches {
		content, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read prestate %s: %w", filepath.Base(filePath), err)
		}

		var data struct {
			Pre string `json:"pre"`
		}
		if err := json.Unmarshal(content, &data); err != nil {
			return nil, fmt.Errorf("failed to parse prestate %s: %w", filepath.Base(filePath), err)
		}

		// Store hash with its corresponding key
		if key, exists := fileToKey[filepath.Base(filePath)]; exists {
			info.Hashes[key] = data.Pre
		}

		// Rename files to hash-based names
		newFileName := data.Pre + ".json"
		hashedPath := filepath.Join(buildDir, newFileName)
		if err := os.Rename(filePath, hashedPath); err != nil {
			return nil, fmt.Errorf("failed to rename prestate %s: %w", filepath.Base(filePath), err)
		}
		log.Printf("%s available at: %s/%s\n", filepath.Base(filePath), prestateURL, newFileName)

		// Rename the corresponding binary file
		binFilePath := strings.Replace(strings.TrimSuffix(filePath, ".json"), "-proof", "", 1) + ".bin.gz"
		newBinFileName := data.Pre + ".bin.gz"
		binHashedPath := filepath.Join(buildDir, newBinFileName)
		if err := os.Rename(binFilePath, binHashedPath); err != nil {
			return nil, fmt.Errorf("failed to rename prestate %s: %w", filepath.Base(binFilePath), err)
		}
		log.Printf("%s available at: %s/%s\n", filepath.Base(binFilePath), prestateURL, newBinFileName)
	}

	h.info = info
	return info, nil
}

func (m *Main) localPrestateOption(dir string) tmpl.TemplateContextOptions {
	holder := newLocalPrestateHolder(m.cfg.baseDir, dir, m.cfg.dryRun)

	return tmpl.WithFunction("localPrestate", func() (*PrestateInfo, error) {
		return holder.GetPrestateInfo()
	})
}

func (m *Main) renderTemplate(dir string) (*bytes.Buffer, error) {
	opts := []tmpl.TemplateContextOptions{
		m.localDockerImageOption(),
		m.localContractArtifactsOption(dir),
		m.localPrestateOption(dir),
		tmpl.WithBaseDir(m.cfg.baseDir),
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

	spec, err := d.Deploy(ctx, buf)
	if err != nil {
		return fmt.Errorf("error deploying kurtosis package: %w", err)
	}

	env, err := d.GetEnvironmentInfo(ctx, spec)
	if err != nil {
		return fmt.Errorf("error getting environment: %w", err)
	}

	if err := writeEnvironment(m.cfg.environment, env); err != nil {
		return fmt.Errorf("error writing environment: %w", err)
	}

	return nil
}

func (m *Main) deployFileserver(ctx context.Context, sourceDir string) error {
	// Create a temp dir in the fileserver package
	baseDir := filepath.Join(m.cfg.baseDir, FILESERVER_PACKAGE)
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return fmt.Errorf("error creating base directory: %w", err)
	}
	tempDir, err := os.MkdirTemp(baseDir, "upload-content")
	if err != nil {
		return fmt.Errorf("error creating temporary directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Copy build dir contents to tempDir
	if err := util.CopyDir(sourceDir, tempDir); err != nil {
		return fmt.Errorf("error copying directory: %w", err)
	}

	buf := bytes.NewBuffer(nil)
	buf.WriteString(fmt.Sprintf("source_path: %s\n", filepath.Base(tempDir)))

	opts := []kurtosis.KurtosisDeployerOptions{
		kurtosis.WithKurtosisBaseDir(m.cfg.baseDir),
		kurtosis.WithKurtosisDryRun(m.cfg.dryRun),
		kurtosis.WithKurtosisPackageName(FILESERVER_PACKAGE),
		kurtosis.WithKurtosisEnclave(m.cfg.enclave),
	}

	d, err := m.newDeployer(opts...)
	if err != nil {
		return fmt.Errorf("error creating kurtosis deployer: %w", err)
	}

	_, err = d.Deploy(ctx, buf)
	if err != nil {
		return fmt.Errorf("error deploying kurtosis package: %w", err)
	}

	return nil
}

type deployer interface {
	Deploy(ctx context.Context, input io.Reader) (*spec.EnclaveSpec, error)
	GetEnvironmentInfo(ctx context.Context, spec *spec.EnclaveSpec) (*kurtosis.KurtosisEnvironment, error)
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

	tmpDir, err := os.MkdirTemp("", m.cfg.enclave)
	if err != nil {
		return fmt.Errorf("error creating temporary directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	buf, err := m.renderTemplate(tmpDir)
	if err != nil {
		return fmt.Errorf("error rendering template: %w", err)
	}

	// TODO: clean up consumers of static server and replace with fileserver
	err = m.deployFileserver(ctx, tmpDir)
	if err != nil {
		return fmt.Errorf("error deploying fileserver: %w", err)
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
