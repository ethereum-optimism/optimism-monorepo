package kurtosis

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

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
			d := NewKurtosisDeployer(tt.opts...)
			assert.Equal(t, tt.wantBaseDir, d.baseDir)
			assert.Equal(t, tt.wantPkg, d.packageName)
			assert.Equal(t, tt.wantDryRun, d.dryRun)
			assert.Equal(t, tt.wantEnclave, d.enclave)
		})
	}
}

func TestPrepareArgFile(t *testing.T) {
	d := NewKurtosisDeployer()
	input := strings.NewReader("test content")

	path, err := d.prepareArgFile(input)
	require.NoError(t, err)
	defer func() {
		err := os.Remove(path)
		require.NoError(t, err)
	}()

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "test content", string(content))
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
	testSpecWithL2 := &spec.EnclaveSpec{
		Chains: []spec.ChainSpec{
			{
				Name:      "op-kurtosis",
				NetworkID: "1234",
			},
		},
	}

	testSpecNoL2 := &spec.EnclaveSpec{
		Chains: []spec.ChainSpec{},
	}

	// Define successful responses that will be used in multiple test cases
	successResponses := []fakeStarlarkResponse{
		{progressMsg: []string{"Starting deployment..."}},
		{info: "Preparing environment"},
		{instruction: "Executing package"},
		{progressMsg: []string{"Deployment complete"}},
		{isSuccessful: true},
	}

	testServices := make(inspect.ServiceMap)
	testServices["el-1-geth-lighthouse"] = inspect.PortMap{
		"rpc": {Port: 52645},
	}
	testServices["op-el-1-op-geth-op-node-op-kurtosis"] = inspect.PortMap{
		"rpc": {Port: 53402},
	}
	testServices["op-cl-1-op-node-op-geth-op-kurtosis"] = inspect.PortMap{
		"http": {Port: 53503},
	}
	testServices["op-batcher-op-kurtosis"] = inspect.PortMap{
		"http": {Port: 53572},
	}

	testWallets := deployer.WalletList{
		{
			Name:       "test-wallet",
			Address:    "0x123",
			PrivateKey: "0xabc",
		},
	}

	testAddresses := deployer.DeploymentAddresses{
		"contract1": "0xdef",
	}

	tests := []struct {
		name           string
		input          string
		spec           *spec.EnclaveSpec
		specErr        error
		inspectResult  *inspect.InspectData
		inspectErr     error
		deployerState  *deployer.DeployerData
		deployerErr    error
		kurtosisErr    error
		responses      []fakeStarlarkResponse
		wantL1Nodes    []Node
		wantL2Nodes    []Node
		wantL2Services EndpointMap
		wantWallets    WalletMap
		wantErr        bool
	}{
		{
			name:  "successful deployment",
			input: "test input",
			spec:  testSpecWithL2,
			inspectResult: &inspect.InspectData{
				UserServices: testServices,
			},
			deployerState: &deployer.DeployerData{
				Wallets: testWallets,
				State: map[string]deployer.DeploymentAddresses{
					"1234": testAddresses,
				},
			},
			responses: successResponses,
			wantL1Nodes: []Node{
				{
					"el": "http://localhost:52645",
				},
			},
			wantL2Nodes: []Node{
				{
					"el": "http://localhost:53402",
					"cl": "http://localhost:53503",
				},
			},
			wantL2Services: EndpointMap{
				"batcher": "http://localhost:53572",
			},
			wantWallets: WalletMap{
				"test-wallet": {
					Address:    "0x123",
					PrivateKey: "0xabc",
				},
			},
		},
		{
			name:    "spec error",
			input:   "test input",
			spec:    testSpecWithL2,
			specErr: fmt.Errorf("spec failed"),
			wantErr: true,
		},
		{
			name:       "inspect error",
			input:      "test input",
			spec:       testSpecWithL2,
			inspectErr: fmt.Errorf("inspect failed"),
			wantErr:    true,
		},
		{
			name:        "kurtosis error",
			input:       "test input",
			spec:        testSpecWithL2,
			kurtosisErr: fmt.Errorf("kurtosis failed"),
			wantErr:     true,
		},
		{
			name:  "deployer error",
			input: "test input",
			spec:  testSpecWithL2,
			inspectResult: &inspect.InspectData{
				UserServices: testServices,
			},
			deployerErr: fmt.Errorf("deployer failed"),
			wantErr:     true,
		},
		{
			name:  "successful deployment with no L1",
			input: "test input",
			spec:  testSpecWithL2,
			inspectResult: &inspect.InspectData{
				UserServices: inspect.ServiceMap{
					"op-el-1-op-geth-op-node-op-kurtosis": inspect.PortMap{
						"rpc": {Port: 53402},
					},
					"op-cl-1-op-node-op-geth-op-kurtosis": inspect.PortMap{
						"http": {Port: 53503},
					},
				},
			},
			deployerState: &deployer.DeployerData{
				Wallets: testWallets,
				State: map[string]deployer.DeploymentAddresses{
					"1234": testAddresses,
				},
			},
			responses: successResponses,
			wantL2Nodes: []Node{
				{
					"el": "http://localhost:53402",
					"cl": "http://localhost:53503",
				},
			},
			wantWallets: WalletMap{
				"test-wallet": {
					Address:    "0x123",
					PrivateKey: "0xabc",
				},
			},
		},
		{
			name:  "successful deployment with no L2",
			input: "test input",
			spec:  testSpecNoL2,
			inspectResult: &inspect.InspectData{
				UserServices: inspect.ServiceMap{
					"el-1-geth-lighthouse": inspect.PortMap{
						"rpc": {Port: 52645},
					},
				},
			},
			deployerState: &deployer.DeployerData{
				Wallets: testWallets,
			},
			responses: successResponses,
			wantL1Nodes: []Node{
				{
					"el": "http://localhost:52645",
				},
			},
			wantWallets: WalletMap{
				"test-wallet": {
					Address:    "0x123",
					PrivateKey: "0xabc",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fake Kurtosis context
			fakeCtx := &fakeKurtosisContext{
				enclaveCtx: &fakeEnclaveContext{
					runErr:    tt.kurtosisErr,
					responses: tt.responses,
				},
			}

			d := NewKurtosisDeployer(
				WithKurtosisEnclaveSpec(&fakeEnclaveSpecifier{
					spec: tt.spec,
					err:  tt.specErr,
				}),
				WithKurtosisEnclaveInspecter(&fakeEnclaveInspecter{
					result: tt.inspectResult,
					err:    tt.inspectErr,
				}),
				WithKurtosisEnclaveObserver(&fakeEnclaveObserver{
					state: tt.deployerState,
					err:   tt.deployerErr,
				}),
			)

			// Set the fake Kurtosis context
			d.kurtosisCtx = fakeCtx

			env, err := d.Deploy(context.Background(), strings.NewReader(tt.input))
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, env)

			if tt.wantL1Nodes != nil {
				require.NotNil(t, env.L1)
				assert.Equal(t, tt.wantL1Nodes, env.L1.Nodes)
			} else {
				assert.Nil(t, env.L1)
			}

			if tt.wantL2Nodes != nil {
				require.Len(t, env.L2, 1)
				assert.Equal(t, tt.wantL2Nodes, env.L2[0].Nodes)
				if tt.wantL2Services != nil {
					assert.Equal(t, tt.wantL2Services, env.L2[0].Services)
				}
			} else {
				assert.Empty(t, env.L2)
			}

			assert.Equal(t, tt.wantWallets, env.Wallets)
		})
	}
}
