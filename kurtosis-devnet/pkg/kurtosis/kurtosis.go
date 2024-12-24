package kurtosis

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"text/template"

	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis/sources/deployer"
	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis/sources/spec"
)

const (
	DefaultPackageName = "github.com/ethpandaops/optimism-package"
	DefaultEnclave     = "devnet"
)

type EndpointMap map[string]string

type Node = EndpointMap

type Chain struct {
	Name      string                       `json:"name"`
	ID        string                       `json:"id,omitempty"`
	Services  EndpointMap                  `json:"services,omitempty"`
	Nodes     []Node                       `json:"nodes"`
	Addresses deployer.DeploymentAddresses `json:"addresses,omitempty"`
}

type Wallet struct {
	Address    string `json:"address"`
	PrivateKey string `json:"private_key,omitempty"`
}

type WalletMap map[string]Wallet

// KurtosisEnvironment represents the output of a Kurtosis deployment
type KurtosisEnvironment struct {
	L1      *Chain    `json:"l1"`
	L2      []*Chain  `json:"l2"`
	Wallets WalletMap `json:"wallets"`
}

// KurtosisDeployer handles deploying packages using Kurtosis
type KurtosisDeployer struct {
	// Base directory where the deployment commands should be executed
	baseDir string
	// Template for the deployment command
	cmdTemplate *template.Template
	// Package name to deploy
	packageName string
	// Dry run mode
	dryRun bool
	// Enclave name
	enclave string

	enclaveSpec      EnclaveSpecifier
	enclaveInspecter EnclaveInspecter
	enclaveObserver  EnclaveObserver
}

const cmdTemplateStr = "just _kurtosis-run {{.PackageName}} {{.ArgFile}} {{.Enclave}}"

var defaultCmdTemplate *template.Template

func init() {
	defaultCmdTemplate = template.Must(template.New("kurtosis_deploy_cmd").Parse(cmdTemplateStr))
}

type KurtosisDeployerOptions func(*KurtosisDeployer)

func WithKurtosisBaseDir(baseDir string) KurtosisDeployerOptions {
	return func(d *KurtosisDeployer) {
		d.baseDir = baseDir
	}
}

func WithKurtosisCmdTemplate(cmdTemplate *template.Template) KurtosisDeployerOptions {
	return func(d *KurtosisDeployer) {
		d.cmdTemplate = cmdTemplate
	}
}

func WithKurtosisPackageName(packageName string) KurtosisDeployerOptions {
	return func(d *KurtosisDeployer) {
		d.packageName = packageName
	}
}

func WithKurtosisDryRun(dryRun bool) KurtosisDeployerOptions {
	return func(d *KurtosisDeployer) {
		d.dryRun = dryRun
	}
}

func WithKurtosisEnclave(enclave string) KurtosisDeployerOptions {
	return func(d *KurtosisDeployer) {
		d.enclave = enclave
	}
}

func WithKurtosisEnclaveSpec(enclaveSpec EnclaveSpecifier) KurtosisDeployerOptions {
	return func(d *KurtosisDeployer) {
		d.enclaveSpec = enclaveSpec
	}
}

func WithKurtosisEnclaveInspecter(enclaveInspecter EnclaveInspecter) KurtosisDeployerOptions {
	return func(d *KurtosisDeployer) {
		d.enclaveInspecter = enclaveInspecter
	}
}

func WithKurtosisEnclaveObserver(enclaveObserver EnclaveObserver) KurtosisDeployerOptions {
	return func(d *KurtosisDeployer) {
		d.enclaveObserver = enclaveObserver
	}
}

// NewKurtosisDeployer creates a new KurtosisDeployer instance
func NewKurtosisDeployer(opts ...KurtosisDeployerOptions) *KurtosisDeployer {
	d := &KurtosisDeployer{
		baseDir:     ".",
		cmdTemplate: defaultCmdTemplate,
		packageName: DefaultPackageName,
		dryRun:      false,
		enclave:     "devnet",

		enclaveSpec:      &enclaveSpecAdapter{},
		enclaveInspecter: &enclaveInspectAdapter{},
		enclaveObserver:  &enclaveDeployerAdapter{},
	}

	for _, opt := range opts {
		opt(d)
	}

	return d
}

// templateData holds the data for the command template
type templateData struct {
	PackageName string
	ArgFile     string
	Enclave     string
}

// prepareArgFile creates a temporary file with the input content and returns its path
// The caller is responsible for deleting the file.
func (d *KurtosisDeployer) prepareArgFile(input io.Reader) (string, error) {
	argFile, err := os.CreateTemp("", "kurtosis-args-*.yaml")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary arg file: %w", err)
	}
	defer argFile.Close()

	if _, err := io.Copy(argFile, input); err != nil {
		os.Remove(argFile.Name())
		return "", fmt.Errorf("failed to write arg file: %w", err)
	}

	return argFile.Name(), nil
}

// runKurtosisCommand executes the kurtosis command with the given arguments
// TODO: reimplement this with the kurtosis SDK, it'll be cleaner.
func (d *KurtosisDeployer) runKurtosisCommand(argFile string) error {
	data := templateData{
		PackageName: d.packageName,
		ArgFile:     argFile,
		Enclave:     d.enclave,
	}

	var cmdBuf bytes.Buffer
	if err := d.cmdTemplate.Execute(&cmdBuf, data); err != nil {
		return fmt.Errorf("failed to execute command template: %w", err)
	}

	if d.dryRun {
		fmt.Println("Dry run mode enabled, kurtosis would run the following command:")
		fmt.Println(cmdBuf.String())
		return nil
	}

	cmd := exec.Command("sh", "-c", cmdBuf.String())
	cmd.Dir = d.baseDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("kurtosis deployment failed: %w", err)
	}

	return nil
}

func (d *KurtosisDeployer) getWallets(wallets deployer.WalletList) WalletMap {
	walletMap := make(WalletMap)
	for _, wallet := range wallets {
		walletMap[wallet.Name] = Wallet{
			Address:    wallet.Address,
			PrivateKey: wallet.PrivateKey,
		}
	}
	return walletMap
}

// getEnvironmentInfo parses the input spec and inspect output to create KurtosisEnvironment
func (d *KurtosisDeployer) getEnvironmentInfo(ctx context.Context, spec *spec.EnclaveSpec) (*KurtosisEnvironment, error) {
	inspectResult, err := d.enclaveInspecter.EnclaveInspect(ctx, d.enclave)
	if err != nil {
		return nil, fmt.Errorf("failed to parse inspect output: %w", err)
	}

	// Get contract addresses
	deployerState, err := d.enclaveObserver.EnclaveObserve(ctx, d.enclave)
	if err != nil {
		return nil, fmt.Errorf("failed to parse deployer state: %w", err)
	}

	env := &KurtosisEnvironment{
		L2:      make([]*Chain, 0, len(spec.Chains)),
		Wallets: d.getWallets(deployerState.Wallets),
	}

	// Find L1 endpoint
	finder := NewServiceFinder(inspectResult.UserServices)
	if nodes, endpoints := finder.FindL1Endpoints(); len(nodes) > 0 {
		env.L1 = &Chain{
			Name:     "Ethereum",
			Services: endpoints,
			Nodes:    nodes,
		}
	}

	// Find L2 endpoints
	for _, chainSpec := range spec.Chains {
		nodes, endpoints := finder.FindL2Endpoints(chainSpec.Name)

		chain := &Chain{
			Name:     chainSpec.Name,
			ID:       chainSpec.NetworkID,
			Services: endpoints,
			Nodes:    nodes,
		}

		// Add contract addresses if available
		if addresses, ok := deployerState.State[chainSpec.NetworkID]; ok {
			chain.Addresses = addresses
		}

		env.L2 = append(env.L2, chain)
	}

	return env, nil
}

// Deploy executes the Kurtosis deployment command with the provided input
func (d *KurtosisDeployer) Deploy(ctx context.Context, input io.Reader) (*KurtosisEnvironment, error) {
	// Parse the input spec first
	inputCopy := new(bytes.Buffer)
	tee := io.TeeReader(input, inputCopy)

	spec, err := d.enclaveSpec.EnclaveSpec(tee)
	if err != nil {
		return nil, fmt.Errorf("failed to parse input spec: %w", err)
	}

	// Prepare argument file
	argFile, err := d.prepareArgFile(inputCopy)
	if err != nil {
		return nil, err
	}
	defer os.Remove(argFile)

	// Run kurtosis command
	if err := d.runKurtosisCommand(argFile); err != nil {
		return nil, err
	}

	// If dry run, return empty environment
	if d.dryRun {
		return &KurtosisEnvironment{}, nil
	}

	// Get environment information
	return d.getEnvironmentInfo(ctx, spec)
}
