package kurtosis

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis/api/fake"
	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis/api/interfaces"
	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis/sources/deployer"
	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis/sources/inspect"
	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis/sources/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKurtosisDeployer(t *testing.T) {
	tests := []struct {
		name        string
		opts        []KurtosisDeployerOptions
		wantBaseDir string
		wantPkg     string
		wantDryRun  bool
		wantEnclave string
	}{
		{
			name:        "default values",
			opts:        nil,
			wantBaseDir: ".",
			wantPkg:     DefaultPackageName,
			wantDryRun:  false,
			wantEnclave: "devnet",
		},
		{
			name: "with options",
			opts: []KurtosisDeployerOptions{
				WithKurtosisBaseDir("/custom/dir"),
				WithKurtosisPackageName("custom-package"),
				WithKurtosisDryRun(true),
				WithKurtosisEnclave("custom-enclave"),
			},
			wantBaseDir: "/custom/dir",
			wantPkg:     "custom-package",
			wantDryRun:  true,
			wantEnclave: "custom-enclave",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, err := NewKurtosisDeployer(tt.opts...)
			require.NoError(t, err)
			assert.Equal(t, tt.wantBaseDir, d.baseDir)
			assert.Equal(t, tt.wantPkg, d.packageName)
			assert.Equal(t, tt.wantDryRun, d.dryRun)
			assert.Equal(t, tt.wantEnclave, d.enclave)
		})
	}
}

// fakeEnclaveInspecter implements EnclaveInspecter for testing
type fakeEnclaveInspecter struct {
	result *inspect.InspectData
	err    error
}

func (f *fakeEnclaveInspecter) EnclaveInspect(ctx context.Context, enclave string) (*inspect.InspectData, error) {
	return f.result, f.err
}

// fakeEnclaveObserver implements EnclaveObserver for testing
type fakeEnclaveObserver struct {
	state *deployer.DeployerData
	err   error
}

func (f *fakeEnclaveObserver) EnclaveObserve(ctx context.Context, enclave string) (*deployer.DeployerData, error) {
	return f.state, f.err
}

// fakeEnclaveSpecifier implements EnclaveSpecifier for testing
type fakeEnclaveSpecifier struct {
	spec *spec.EnclaveSpec
	err  error
}

func (f *fakeEnclaveSpecifier) EnclaveSpec(r io.Reader) (*spec.EnclaveSpec, error) {
	return f.spec, f.err
}

func TestDeploy(t *testing.T) {
	testSpec := &spec.EnclaveSpec{
		Chains: []spec.ChainSpec{
			{
				Name:      "op-kurtosis",
				NetworkID: "1234",
			},
		},
	}

	testServices := make(inspect.ServiceMap)
	testServices["el-1-geth-lighthouse"] = inspect.PortMap{
		"rpc": {Port: 52645},
	}

	testWallets := deployer.WalletList{
		{
			Name:       "test-wallet",
			Address:    "0x123",
			PrivateKey: "0xabc",
		},
	}

	tests := []struct {
		name        string
		specErr     error
		inspectErr  error
		deployerErr error
		kurtosisErr error
		wantErr     bool
	}{
		{
			name: "successful deployment",
		},
		{
			name:    "spec error",
			specErr: fmt.Errorf("spec failed"),
			wantErr: true,
		},
		{
			name:       "inspect error",
			inspectErr: fmt.Errorf("inspect failed"),
			wantErr:    true,
		},
		{
			name:        "kurtosis error",
			kurtosisErr: fmt.Errorf("kurtosis failed"),
			wantErr:     true,
		},
		{
			name:        "deployer error",
			deployerErr: fmt.Errorf("deployer failed"),
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fake Kurtosis context that will return the test error
			fakeCtx := &fake.KurtosisContext{
				EnclaveCtx: &fake.EnclaveContext{
					RunErr: tt.kurtosisErr,
					// Send a successful run finished event for successful cases
					Responses: []interfaces.StarlarkResponse{
						&fake.StarlarkResponse{
							IsSuccessful: !tt.wantErr,
						},
					},
				},
			}

			d, err := NewKurtosisDeployer(
				WithKurtosisEnclaveSpec(&fakeEnclaveSpecifier{
					spec: testSpec,
					err:  tt.specErr,
				}),
				WithKurtosisEnclaveInspecter(&fakeEnclaveInspecter{
					result: &inspect.InspectData{
						UserServices: testServices,
					},
					err: tt.inspectErr,
				}),
				WithKurtosisEnclaveObserver(&fakeEnclaveObserver{
					state: &deployer.DeployerData{
						Wallets: testWallets,
					},
					err: tt.deployerErr,
				}),
				WithKurtosisKurtosisContext(fakeCtx),
			)
			require.NoError(t, err)

			_, err = d.Deploy(context.Background(), strings.NewReader("test input"))
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
		})
	}
}
