package kurtosis

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"text/template"

	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis/sources/deployer"
	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis/sources/inspect"
	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis/sources/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindRPCEndpoints(t *testing.T) {
	testServices := inspect.ServiceMap{
		"el-1-geth-lighthouse": {
			"metrics":       52643,
			"tcp-discovery": 52644,
			"udp-discovery": 51936,
			"engine-rpc":    52642,
			"rpc":           52645,
			"ws":            52646,
		},
		"op-batcher-op-kurtosis": {
			"http": 53572,
		},
		"op-cl-1-op-node-op-geth-op-kurtosis": {
			"udp-discovery": 50990,
			"http":          53503,
			"tcp-discovery": 53504,
		},
		"op-el-1-op-geth-op-node-op-kurtosis": {
			"udp-discovery": 53233,
			"engine-rpc":    53399,
			"metrics":       53400,
			"rpc":           53402,
			"ws":            53403,
			"tcp-discovery": 53401,
		},
		"vc-1-geth-lighthouse": {
			"metrics": 53149,
		},
		"cl-1-lighthouse-geth": {
			"metrics":       52691,
			"tcp-discovery": 52692,
			"udp-discovery": 58275,
			"http":          52693,
		},
	}

	tests := []struct {
		name          string
		services      inspect.ServiceMap
		lookupFn      func(inspect.ServiceMap) ([]Node, EndpointMap)
		wantNodes     []Node
		wantEndpoints EndpointMap
	}{
		{
			name:     "find L1 endpoints",
			services: testServices,
			lookupFn: findL1Endpoints,
			wantNodes: []Node{
				{
					"cl": "http://localhost:52693",
					"el": "http://localhost:52645",
				},
			},
			wantEndpoints: EndpointMap{},
		},
		{
			name:     "find op-kurtosis L2 endpoints",
			services: testServices,
			lookupFn: func(services inspect.ServiceMap) ([]Node, EndpointMap) {
				return findL2Endpoints(services, "-op-kurtosis")
			},
			wantNodes: []Node{
				{
					"cl": "http://localhost:53503",
					"el": "http://localhost:53402",
				},
			},
			wantEndpoints: EndpointMap{
				"batcher": "http://localhost:53572",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotNodes, gotEndpoints := tt.lookupFn(tt.services)
			assert.Equal(t, tt.wantNodes, gotNodes)
			assert.Equal(t, tt.wantEndpoints, gotEndpoints)
		})
	}
}

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

func TestRunKurtosisCommand(t *testing.T) {
	fakeCmdTemplate := template.Must(template.New("fake_cmd").Parse("echo 'would run: {{.PackageName}} {{.ArgFile}} {{.Enclave}}'"))

	tests := []struct {
		name        string
		dryRun      bool
		wantError   bool
		wantOutput  bool
		cmdTemplate *template.Template
	}{
		{
			name:        "dry run",
			dryRun:      true,
			wantError:   false,
			cmdTemplate: fakeCmdTemplate,
		},
		{
			name:        "successful run",
			dryRun:      false,
			wantError:   false,
			wantOutput:  true,
			cmdTemplate: fakeCmdTemplate,
		},
		{
			name:        "template error",
			dryRun:      false,
			wantError:   true,
			cmdTemplate: template.Must(template.New("bad_cmd").Parse("{{.NonExistentField}}")),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := NewKurtosisDeployer(
				WithKurtosisDryRun(tt.dryRun),
				WithKurtosisCmdTemplate(tt.cmdTemplate),
			)
			err := d.runKurtosisCommand("test.yaml")
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
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
	// Create a template that just echoes the command that would be run
	fakeCmdTemplate := template.Must(template.New("fake_cmd").Parse("echo 'would run: {{.PackageName}} {{.ArgFile}} {{.Enclave}}'"))

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

	testServices := inspect.ServiceMap{
		"el-1-geth-lighthouse": {
			"rpc": 52645,
		},
		"op-el-1-op-geth-op-node-op-kurtosis": {
			"rpc": 53402,
		},
		"op-cl-1-op-node-op-geth-op-kurtosis": {
			"http": 53503,
		},
		"op-batcher-op-kurtosis": {
			"http": 53572,
		},
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
		dryRun         bool
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
			name:   "dry run",
			input:  "test input",
			spec:   testSpecWithL2,
			dryRun: true,
		},
		{
			name:       "inspect error",
			input:      "test input",
			spec:       testSpecWithL2,
			inspectErr: fmt.Errorf("inspect failed"),
			wantErr:    true,
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
					"op-el-1-op-geth-op-node-op-kurtosis": {
						"rpc": 53402,
					},
					"op-cl-1-op-node-op-geth-op-kurtosis": {
						"http": 53503,
					},
				},
			},
			deployerState: &deployer.DeployerData{
				Wallets: testWallets,
				State: map[string]deployer.DeploymentAddresses{
					"1234": testAddresses,
				},
			},
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
					"el-1-geth-lighthouse": {
						"rpc": 52645,
					},
				},
			},
			deployerState: &deployer.DeployerData{
				Wallets: testWallets,
			},
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
			d := NewKurtosisDeployer(
				WithKurtosisDryRun(tt.dryRun),
				WithKurtosisCmdTemplate(fakeCmdTemplate),
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

			env, err := d.Deploy(context.Background(), strings.NewReader(tt.input))
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			if tt.dryRun {
				assert.NotNil(t, env)
				assert.Empty(t, env.L1)
				assert.Empty(t, env.L2)
				assert.Empty(t, env.Wallets)
				return
			}

			if tt.wantL1Nodes != nil {
				assert.Equal(t, tt.wantL1Nodes, env.L1.Nodes)
			} else {
				assert.Nil(t, env.L1)
			}
			if len(tt.wantL2Nodes) > 0 {
				assert.Equal(t, tt.wantL2Nodes, env.L2[0].Nodes)
				if tt.wantL2Services != nil {
					assert.Equal(t, tt.wantL2Services, env.L2[0].Services)
				}
				if addresses, ok := tt.deployerState.State["1234"]; ok {
					assert.Equal(t, addresses, env.L2[0].Addresses)
				}
			} else {
				assert.Empty(t, env.L2)
			}
			assert.Equal(t, tt.wantWallets, env.Wallets)
		})
	}
}
