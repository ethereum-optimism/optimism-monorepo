package kurtosis

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis/api/interfaces"
	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis/api/run"
	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis/api/wrappers"
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
	Wallets   WalletMap                    `json:"wallets,omitempty"`
}

type Wallet struct {
	Address    string `json:"address"`
	PrivateKey string `json:"private_key,omitempty"`
}

type WalletMap map[string]Wallet

// KurtosisEnvironment represents the output of a Kurtosis deployment
type KurtosisEnvironment struct {
	L1 *Chain   `json:"l1"`
	L2 []*Chain `json:"l2"`
}

// KurtosisDeployer handles deploying packages using Kurtosis
type KurtosisDeployer struct {
	// Base directory where the deployment commands should be executed
	baseDir string
	// Package name to deploy
	packageName string
	// Dry run mode
	dryRun bool
	// Enclave name
	enclave string

	enclaveSpec      EnclaveSpecifier
	enclaveInspecter EnclaveInspecter
	enclaveObserver  EnclaveObserver
	kurtosisCtx      interfaces.KurtosisContextInterface
}

type KurtosisDeployerOptions func(*KurtosisDeployer)

func WithKurtosisBaseDir(baseDir string) KurtosisDeployerOptions {
	return func(d *KurtosisDeployer) {
		d.baseDir = baseDir
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

func WithKurtosisKurtosisContext(kurtosisCtx interfaces.KurtosisContextInterface) KurtosisDeployerOptions {
	return func(d *KurtosisDeployer) {
		d.kurtosisCtx = kurtosisCtx
	}
}

// NewKurtosisDeployer creates a new KurtosisDeployer instance
func NewKurtosisDeployer(opts ...KurtosisDeployerOptions) (*KurtosisDeployer, error) {
	d := &KurtosisDeployer{
		baseDir:     ".",
		packageName: DefaultPackageName,
		dryRun:      false,
		enclave:     DefaultEnclave,

		enclaveSpec:      &enclaveSpecAdapter{},
		enclaveInspecter: &enclaveInspectAdapter{},
		enclaveObserver:  &enclaveDeployerAdapter{},
	}

	for _, opt := range opts {
		opt(d)
	}

	if d.kurtosisCtx == nil {
		var err error
		d.kurtosisCtx, err = wrappers.GetDefaultKurtosisContext()
		if err != nil {
			return nil, fmt.Errorf("failed to create Kurtosis context: %w", err)
		}
	}

	return d, nil
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
		L2: make([]*Chain, 0, len(spec.Chains)),
	}

	// Find L1 endpoint
	finder := NewServiceFinder(inspectResult.UserServices)
	if nodes, endpoints := finder.FindL1Endpoints(); len(nodes) > 0 {
		chain := &Chain{
			Name:     "Ethereum",
			Services: endpoints,
			Nodes:    nodes,
		}
		if deployerState.State != nil {
			chain.Addresses = deployerState.State.Addresses
			chain.Wallets = d.getWallets(deployerState.Wallets)
		}
		env.L1 = chain
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
		if deployerState.State != nil && deployerState.State.Deployments != nil {
			if addresses, ok := deployerState.State.Deployments[chainSpec.NetworkID]; ok {
				chain.Addresses = addresses.Addresses
			}
			if wallets, ok := deployerState.State.Deployments[chainSpec.NetworkID]; ok {
				chain.Wallets = d.getWallets(wallets.Wallets)
			}
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

	// Run kurtosis command
	kurtosisRunner, err := run.NewKurtosisRunner(
		run.WithKurtosisRunnerDryRun(d.dryRun),
		run.WithKurtosisRunnerEnclave(d.enclave),
		run.WithKurtosisRunnerKurtosisContext(d.kurtosisCtx),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kurtosis runner: %w", err)
	}

	if err := kurtosisRunner.Run(ctx, d.packageName, inputCopy); err != nil {
		return nil, err
	}

	// If dry run, return empty environment
	if d.dryRun {
		return &KurtosisEnvironment{}, nil
	}

	// Get environment information
	return d.getEnvironmentInfo(ctx, spec)
}
