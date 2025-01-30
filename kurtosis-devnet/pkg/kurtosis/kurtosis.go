package kurtosis

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/ethereum-optimism/optimism/devnet-sdk/descriptors"
	apiInterfaces "github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis/api/interfaces"
	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis/api/run"
	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis/api/wrappers"
	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis/sources/deployer"
	srcInterfaces "github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis/sources/interfaces"
	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis/sources/spec"
)

const (
	DefaultPackageName = "github.com/ethpandaops/optimism-package"
	DefaultEnclave     = "devnet"
)

// KurtosisEnvironment represents the output of a Kurtosis deployment
type KurtosisEnvironment struct {
	descriptors.DevnetEnvironment
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

	// interfaces for kurtosis sources
	enclaveSpec      srcInterfaces.EnclaveSpecifier
	enclaveInspecter srcInterfaces.EnclaveInspecter
	enclaveObserver  srcInterfaces.EnclaveObserver
	jwtExtractor     srcInterfaces.JWTExtractor

	// interface for kurtosis interactions
	kurtosisCtx apiInterfaces.KurtosisContextInterface
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

func WithKurtosisEnclaveSpec(enclaveSpec srcInterfaces.EnclaveSpecifier) KurtosisDeployerOptions {
	return func(d *KurtosisDeployer) {
		d.enclaveSpec = enclaveSpec
	}
}

func WithKurtosisEnclaveInspecter(enclaveInspecter srcInterfaces.EnclaveInspecter) KurtosisDeployerOptions {
	return func(d *KurtosisDeployer) {
		d.enclaveInspecter = enclaveInspecter
	}
}

func WithKurtosisEnclaveObserver(enclaveObserver srcInterfaces.EnclaveObserver) KurtosisDeployerOptions {
	return func(d *KurtosisDeployer) {
		d.enclaveObserver = enclaveObserver
	}
}

func WithKurtosisJWTExtractor(extractor srcInterfaces.JWTExtractor) KurtosisDeployerOptions {
	return func(d *KurtosisDeployer) {
		d.jwtExtractor = extractor
	}
}

func WithKurtosisKurtosisContext(kurtosisCtx apiInterfaces.KurtosisContextInterface) KurtosisDeployerOptions {
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
		jwtExtractor:     &enclaveJWTAdapter{},
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

func (d *KurtosisDeployer) getWallets(wallets deployer.WalletList) descriptors.WalletMap {
	walletMap := make(descriptors.WalletMap)
	for _, wallet := range wallets {
		walletMap[wallet.Name] = descriptors.Wallet{
			Address:    wallet.Address,
			PrivateKey: wallet.PrivateKey,
		}
	}
	return walletMap
}

// GetEnvironmentInfo parses the input spec and inspect output to create KurtosisEnvironment
func (d *KurtosisDeployer) GetEnvironmentInfo(ctx context.Context, spec *spec.EnclaveSpec) (*KurtosisEnvironment, error) {
	inspectResult, err := d.enclaveInspecter.EnclaveInspect(ctx, d.enclave)
	if err != nil {
		return nil, fmt.Errorf("failed to parse inspect output: %w", err)
	}

	// Get contract addresses
	deployerState, err := d.enclaveObserver.EnclaveObserve(ctx, d.enclave)
	if err != nil {
		return nil, fmt.Errorf("failed to parse deployer state: %w", err)
	}

	// Get JWT data
	jwtData, err := d.jwtExtractor.ExtractData(ctx, d.enclave)
	if err != nil {
		return nil, fmt.Errorf("failed to extract JWT data: %w", err)
	}

	env := &KurtosisEnvironment{
		DevnetEnvironment: descriptors.DevnetEnvironment{
			L2:       make([]*descriptors.Chain, 0, len(spec.Chains)),
			Features: spec.Features,
		},
	}

	// Find L1 endpoint
	finder := NewServiceFinder(inspectResult.UserServices)
	if nodes, services := finder.FindL1Services(); len(nodes) > 0 {
		chain := &descriptors.Chain{
			Name:     "Ethereum",
			Services: services,
			Nodes:    nodes,
			JWT:      jwtData.L1JWT,
		}
		if deployerState.State != nil {
			chain.Addresses = descriptors.AddressMap(deployerState.State.Addresses)
			chain.Wallets = d.getWallets(deployerState.Wallets)
		}
		env.L1 = chain
	}

	// Find L2 endpoints
	for _, chainSpec := range spec.Chains {
		nodes, services := finder.FindL2Services(chainSpec.Name)

		chain := &descriptors.Chain{
			Name:     chainSpec.Name,
			ID:       chainSpec.NetworkID,
			Services: services,
			Nodes:    nodes,
			JWT:      jwtData.L2JWT,
		}

		// Add contract addresses if available
		if deployerState.State != nil && deployerState.State.Deployments != nil {
			if addresses, ok := deployerState.State.Deployments[chainSpec.NetworkID]; ok {
				chain.Addresses = descriptors.AddressMap(addresses.Addresses)
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
func (d *KurtosisDeployer) Deploy(ctx context.Context, input io.Reader) (*spec.EnclaveSpec, error) {
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
		return spec, nil
	}

	// Get environment information
	return spec, nil
}
