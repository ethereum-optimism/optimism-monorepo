package deploy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis"
	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis/api/engine"
	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis/sources/spec"
)

type EngineManager interface {
	EnsureRunning() error
}

type deployer interface {
	Deploy(ctx context.Context, input io.Reader) (*spec.EnclaveSpec, error)
	GetEnvironmentInfo(ctx context.Context, spec *spec.EnclaveSpec) (*kurtosis.KurtosisEnvironment, error)
}

type DeployerFunc func(opts ...kurtosis.KurtosisDeployerOptions) (deployer, error)

type DeployerOption func(*Deployer)

type Deployer struct {
	baseDir        string
	dryRun         bool
	kurtosisPkg    string
	enclave        string
	kurtosisBinary string
	ktDeployer     DeployerFunc
	engineManager  EngineManager
	templateFile   string
	dataFile       string
}

func WithKurtosisDeployer(ktDeployer DeployerFunc) DeployerOption {
	return func(d *Deployer) {
		d.ktDeployer = ktDeployer
	}
}

func WithEngineManager(engineManager EngineManager) DeployerOption {
	return func(d *Deployer) {
		d.engineManager = engineManager
	}
}

func WithKurtosisBinary(kurtosisBinary string) DeployerOption {
	return func(d *Deployer) {
		d.kurtosisBinary = kurtosisBinary
	}
}

func WithKurtosisPackage(kurtosisPkg string) DeployerOption {
	return func(d *Deployer) {
		d.kurtosisPkg = kurtosisPkg
	}
}

func WithTemplateFile(templateFile string) DeployerOption {
	return func(d *Deployer) {
		d.templateFile = templateFile
	}
}

func WithDataFile(dataFile string) DeployerOption {
	return func(d *Deployer) {
		d.dataFile = dataFile
	}
}

func WithBaseDir(baseDir string) DeployerOption {
	return func(d *Deployer) {
		d.baseDir = baseDir
	}
}

func WithDryRun(dryRun bool) DeployerOption {
	return func(d *Deployer) {
		d.dryRun = dryRun
	}
}

func WithEnclave(enclave string) DeployerOption {
	return func(d *Deployer) {
		d.enclave = enclave
	}
}

func NewDeployer(opts ...DeployerOption) *Deployer {
	d := &Deployer{
		kurtosisBinary: "kurtosis",
		ktDeployer: func(opts ...kurtosis.KurtosisDeployerOptions) (deployer, error) {
			return kurtosis.NewKurtosisDeployer(opts...)
		},
	}
	for _, opt := range opts {
		opt(d)
	}

	if d.engineManager == nil {
		d.engineManager = engine.NewEngineManager(engine.WithKurtosisBinary(d.kurtosisBinary))
	}
	return d
}

func (d *Deployer) deployEnvironment(ctx context.Context, r io.Reader) (*kurtosis.KurtosisEnvironment, error) {
	// Create a multi reader to output deployment input to stdout
	buf := bytes.NewBuffer(nil)
	tee := io.TeeReader(r, buf)

	// Log the deployment input
	log.Println("Deployment input:")
	if _, err := io.Copy(os.Stdout, tee); err != nil {
		return nil, fmt.Errorf("error copying deployment input: %w", err)
	}

	opts := []kurtosis.KurtosisDeployerOptions{
		kurtosis.WithKurtosisBaseDir(d.baseDir),
		kurtosis.WithKurtosisDryRun(d.dryRun),
		kurtosis.WithKurtosisPackageName(d.kurtosisPkg),
		kurtosis.WithKurtosisEnclave(d.enclave),
	}

	ktd, err := d.ktDeployer(opts...)
	if err != nil {
		return nil, fmt.Errorf("error creating kurtosis deployer: %w", err)
	}

	spec, err := ktd.Deploy(ctx, buf)
	if err != nil {
		return nil, fmt.Errorf("error deploying kurtosis package: %w", err)
	}

	return ktd.GetEnvironmentInfo(ctx, spec)
}

func (d *Deployer) renderTemplate(buildDir string, urlBuilder func(path ...string) string) (*bytes.Buffer, error) {
	t := &Templater{
		baseDir:      d.baseDir,
		dryRun:       d.dryRun,
		enclave:      d.enclave,
		templateFile: d.templateFile,
		dataFile:     d.dataFile,
		buildDir:     buildDir,
		urlBuilder:   urlBuilder,
	}

	return t.Render()
}

func (d *Deployer) Deploy(ctx context.Context, r io.Reader) (*kurtosis.KurtosisEnvironment, error) {
	if !d.dryRun {
		if err := d.engineManager.EnsureRunning(); err != nil {
			return nil, fmt.Errorf("error ensuring kurtosis engine is running: %w", err)
		}
	}

	tmpDir, err := os.MkdirTemp("", d.enclave)
	if err != nil {
		return nil, fmt.Errorf("error creating temporary directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	srv := &FileServer{
		baseDir:  d.baseDir,
		dryRun:   d.dryRun,
		enclave:  d.enclave,
		deployer: d.ktDeployer,
	}

	buf, err := d.renderTemplate(tmpDir, srv.URL)
	if err != nil {
		return nil, fmt.Errorf("error rendering template: %w", err)
	}

	if err := srv.Deploy(ctx, tmpDir); err != nil {
		return nil, fmt.Errorf("error deploying fileserver: %w", err)
	}

	return d.deployEnvironment(ctx, buf)
}
