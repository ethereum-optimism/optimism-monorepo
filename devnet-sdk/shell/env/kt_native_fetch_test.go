package env

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"testing"

	"github.com/ethereum-optimism/optimism/devnet-sdk/descriptors"
	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis"
	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis/sources/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	//go:embed testdata/kurtosis
	kurtosisTestData embed.FS
)

func TestParseKurtosisNativeURL(t *testing.T) {
	tests := []struct {
		name           string
		urlStr         string
		wantEnclave    string
		wantArtifact   string
		wantFile       string
		wantParseError bool
	}{
		{
			name:        "absolute file path",
			urlStr:      "ktnative://myenclave/path/args.yaml",
			wantEnclave: "myenclave",
			wantFile:    "/path/args.yaml",
		},
		{
			name:           "invalid url",
			urlStr:         "://invalid",
			wantParseError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := url.Parse(tt.urlStr)
			if tt.wantParseError {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)

			enclave, argsFile := parseKurtosisNativeURL(u)
			assert.Equal(t, tt.wantEnclave, enclave)
			assert.Equal(t, tt.wantFile, argsFile)
		})
	}
}

func TestFetchKurtosisNativeDataFailures(t *testing.T) {
	url, err := url.Parse("ktnative://enclave/file/path")
	require.NoError(t, err)

	t.Run("non-existent args file", func(t *testing.T) {
		osImpl := &mockOSImpl{
			err: fmt.Errorf("oh no"),
		}

		_, _, err = fetchKurtosisNativeDataInternal(url, osImpl, &defaultSpecImpl{}, &defaultKurtosisImpl{})
		require.ErrorContains(t, err, "error reading arguments file: oh no")
	})

	t.Run("malformed args file", func(t *testing.T) {
		file, err := kurtosisTestData.Open("testdata/kurtosis/args--malformed.txt")
		require.NoError(t, err)

		osImpl := &mockOSImpl{
			value: file,
		}

		_, _, err = fetchKurtosisNativeDataInternal(url, osImpl, &defaultSpecImpl{}, &defaultKurtosisImpl{})
		require.ErrorContains(t, err, "error extracting enclave spec: failed to decode YAML: yaml: unmarshal errors:")
	})

	t.Run("spec extraction failure", func(t *testing.T) {
		file, err := kurtosisTestData.Open("testdata/kurtosis/args--simple.yaml")
		require.NoError(t, err)

		osImpl := &mockOSImpl{
			value: file,
		}

		specImpl := &mockSpecImpl{
			err: fmt.Errorf("oh no"),
		}

		_, _, err = fetchKurtosisNativeDataInternal(url, osImpl, specImpl, &defaultKurtosisImpl{})
		require.ErrorContains(t, err, "error extracting enclave spec: oh no")
	})

	t.Run("kurtosis deployer failure", func(t *testing.T) {
		file, err := kurtosisTestData.Open("testdata/kurtosis/args--simple.yaml")
		require.NoError(t, err)

		osImpl := &mockOSImpl{
			value: file,
		}

		kurtosisImpl := &mockKurtosisImpl{
			err: fmt.Errorf("oh no"),
		}

		_, _, err = fetchKurtosisNativeDataInternal(url, osImpl, &defaultSpecImpl{}, kurtosisImpl)
		require.ErrorContains(t, err, "error creating deployer: oh no")
	})

	t.Run("kurtosis info extraction failure", func(t *testing.T) {
		file, err := kurtosisTestData.Open("testdata/kurtosis/args--simple.yaml")
		require.NoError(t, err)

		osImpl := &mockOSImpl{
			value: file,
		}

		kurtosisDeployer := &mockKurtosisDeployerImpl{
			err: fmt.Errorf("oh no"),
		}

		kurtosisImpl := &mockKurtosisImpl{
			value: kurtosisDeployer,
		}

		_, _, err = fetchKurtosisNativeDataInternal(url, osImpl, &defaultSpecImpl{}, kurtosisImpl)
		require.ErrorContains(t, err, "error getting environment info: oh no")
	})
}

func TestFetchKurtosisNativeDataSuccess(t *testing.T) {
	url, err := url.Parse("ktnative://enclave/file/path")
	require.NoError(t, err)

	t.Run("fetching success", func(t *testing.T) {
		file, err := kurtosisTestData.Open("testdata/kurtosis/args--simple.yaml")
		require.NoError(t, err)

		// We'll prepare a mock spec to be returned
		envSpec := &spec.EnclaveSpec{}
		env := &kurtosis.KurtosisEnvironment{
			DevnetEnvironment: descriptors.DevnetEnvironment{
				L2:       make([]*descriptors.Chain, 0, 1),
				Features: envSpec.Features,
			},
		}

		// And serialize it so that we can compare values
		jsonEnv, err := json.MarshalIndent(env, "", "  ")
		require.NoError(t, err)

		osImpl := &mockOSImpl{
			value: file,
		}

		specImpl := &mockSpecImpl{
			value: envSpec,
		}

		kurtosisDeployer := &mockKurtosisDeployerImpl{
			value: env,
		}

		kurtosisImpl := &mockKurtosisImpl{
			value: kurtosisDeployer,
		}

		enclave, jsonDescriptor, err := fetchKurtosisNativeDataInternal(url, osImpl, specImpl, kurtosisImpl)
		require.NoError(t, err)
		require.Equal(t, "enclave", enclave)
		require.Equal(t, jsonDescriptor, jsonEnv)
	})
}

var (
	_ osInterface = (*mockOSImpl)(nil)

	_ specInterface = (*mockSpecImpl)(nil)

	_ kurtosisInterface = (*mockKurtosisImpl)(nil)

	_ kurtosisDeployerInterface = (*mockKurtosisDeployerImpl)(nil)
)

type mockOSImpl struct {
	value fileInterface
	err   error
}

func (o *mockOSImpl) Open(name string) (fileInterface, error) {
	return o.value, o.err
}

type mockSpecImpl struct {
	value *spec.EnclaveSpec
	err   error
}

func (o *mockSpecImpl) ExtractData(r io.Reader) (*spec.EnclaveSpec, error) {
	return o.value, o.err
}

type mockKurtosisImpl struct {
	value kurtosisDeployerInterface
	err   error
}

func (o *mockKurtosisImpl) NewKurtosisDeployer(opts ...kurtosis.KurtosisDeployerOptions) (kurtosisDeployerInterface, error) {
	return o.value, o.err
}

type mockKurtosisDeployerImpl struct {
	value *kurtosis.KurtosisEnvironment
	err   error
}

func (o *mockKurtosisDeployerImpl) GetEnvironmentInfo(context.Context, *spec.EnclaveSpec) (*kurtosis.KurtosisEnvironment, error) {
	return o.value, o.err
}
