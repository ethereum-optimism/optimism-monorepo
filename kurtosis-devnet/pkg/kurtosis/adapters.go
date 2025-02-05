package kurtosis

import (
	"context"
	"io"

	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis/sources/deployer"
	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis/sources/inspect"
	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis/sources/interfaces"
	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis/sources/jwt"
	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis/sources/spec"
)

type enclaveSpecAdapter struct{}

func (a *enclaveSpecAdapter) EnclaveSpec(r io.Reader) (*spec.EnclaveSpec, error) {
	return spec.NewSpec().ExtractData(r)
}

var _ interfaces.EnclaveSpecifier = (*enclaveSpecAdapter)(nil)

type enclaveInspectAdapter struct{}

func (a *enclaveInspectAdapter) EnclaveInspect(ctx context.Context, enclave string) (*inspect.InspectData, error) {
	return inspect.NewInspector(enclave).ExtractData(ctx)
}

var _ interfaces.EnclaveInspecter = (*enclaveInspectAdapter)(nil)

type enclaveDeployerAdapter struct{}

func (a *enclaveDeployerAdapter) EnclaveObserve(ctx context.Context, enclave string) (*deployer.DeployerData, error) {
	return deployer.NewDeployer(enclave).ExtractData(ctx)
}

var _ interfaces.EnclaveObserver = (*enclaveDeployerAdapter)(nil)

type enclaveJWTAdapter struct{}

func (a *enclaveJWTAdapter) ExtractData(ctx context.Context, enclave string) (*jwt.Data, error) {
	return jwt.NewExtractor(enclave).ExtractData(ctx)
}

var _ interfaces.JWTExtractor = (*enclaveJWTAdapter)(nil)
