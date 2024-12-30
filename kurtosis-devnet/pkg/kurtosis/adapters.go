package kurtosis

import (
	"context"
	"io"

	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis/sources/deployer"
	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis/sources/inspect"
	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis/sources/spec"
)

type enclaveSpecAdapter struct{}

func (a *enclaveSpecAdapter) EnclaveSpec(r io.Reader) (*spec.EnclaveSpec, error) {
	return spec.NewSpec().ExtractData(r)
}

var _ EnclaveSpecifier = (*enclaveSpecAdapter)(nil)

type enclaveInspectAdapter struct{}

func (a *enclaveInspectAdapter) EnclaveInspect(ctx context.Context, enclave string) (*inspect.InspectData, error) {
	return inspect.NewInspector(enclave).ExtractData(ctx)
}

var _ EnclaveInspecter = (*enclaveInspectAdapter)(nil)

type enclaveDeployerAdapter struct{}

func (a *enclaveDeployerAdapter) EnclaveObserve(ctx context.Context, enclave string) (*deployer.DeployerData, error) {
	return deployer.NewDeployer(enclave).ExtractData(ctx)
}

var _ EnclaveObserver = (*enclaveDeployerAdapter)(nil)
