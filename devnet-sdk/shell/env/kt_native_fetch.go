package env

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis"
	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis/sources/spec"
)

// parseKurtosisNativeURL parses a Kurtosis URL of the form kt://enclave/artifact/file
// If artifact is omitted, it defaults to "devnet"
// If file is omitted, it defaults to "env.json"
func parseKurtosisNativeURL(u *url.URL) (enclave, argsFileName string) {
	enclave = u.Host
	argsFileName = "/" + strings.Trim(u.Path, "/")

	return
}

// fetchKurtosisNativeData reads data directly from kurtosis API using default dependency implementations
func fetchKurtosisNativeData(u *url.URL) (string, []byte, error) {
	return fetchKurtosisNativeDataInternal(u, &defaultOSOpenImpl{}, &defaultSpecImpl{}, &defaultKurtosisImpl{})
}

// fetchKurtosisNativeDataInternal reads data directly from kurtosis API using provided dependency implementations
func fetchKurtosisNativeDataInternal(u *url.URL, osImpl osOpenInterface, specImpl specInterface, kurtosisImpl kurtosisInterface) (string, []byte, error) {
	// First let's parse the kurtosis URL
	enclave, argsFileName := parseKurtosisNativeURL(u)

	// Open the arguments file
	argsFile, err := osImpl.Open(argsFileName)
	if err != nil {
		return "", nil, fmt.Errorf("error reading arguments file: %w", err)
	}

	// Make sure to close the file once we're done reading
	defer argsFile.Close()

	// Once we have the arguments file, we can extract the enclave spec
	enclaveSpec, err := specImpl.ExtractData(argsFile)
	if err != nil {
		return enclave, nil, fmt.Errorf("error extracting enclave spec: %w", err)
	}

	// We'll use the deployer to extract the system spec
	deployer, err := kurtosisImpl.NewKurtosisDeployer(kurtosis.WithKurtosisEnclave(enclave))
	if err != nil {
		return enclave, nil, fmt.Errorf("error creating deployer: %w", err)
	}

	// We'll read the environment info from kurtosis directly
	ctx := context.Background()
	env, err := deployer.GetEnvironmentInfo(ctx, enclaveSpec)
	if err != nil {
		return enclave, nil, fmt.Errorf("error getting environment info: %w", err)
	}

	// And the last step is to encode this environment as JSON
	envBytes, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return enclave, nil, fmt.Errorf("error converting environment info to JSON: %w", err)
	}

	return enclave, envBytes, nil
}

// osOpenInterface describes a struct that can open filesystem files for reading
//
// osOpenInterface is used when loading kurtosis args files from local filesystem
type osOpenInterface interface {
	Open(name string) (fileInterface, error)
}

// fileInterface describes a subset of os.File struct for ease of testing
type fileInterface interface {
	io.Reader
	Close() error
}

// defaultOSOpenImpl implements osOpenInterface
type defaultOSOpenImpl struct{}

func (d *defaultOSOpenImpl) Open(name string) (fileInterface, error) {
	return os.Open(name)
}

// specInterface describes a subset of functionality required from the spec package
//
// The spec package is responsible for turning a kurtosis args file into an EnclaveSpec
type specInterface interface {
	ExtractData(r io.Reader) (*spec.EnclaveSpec, error)
}

// defaultSpecImpl implements specInterface
type defaultSpecImpl struct{}

func (d *defaultSpecImpl) ExtractData(r io.Reader) (*spec.EnclaveSpec, error) {
	return spec.NewSpec().ExtractData(r)
}

// kurtosisInterface describes a subset of functionality required from the kurtosis package
//
// kurtosisInterface provides access to a KurtosisDeployer object, an intermediate object that provides
// access to the KurtosisEnvironment object
type kurtosisInterface interface {
	NewKurtosisDeployer(opts ...kurtosis.KurtosisDeployerOptions) (kurtosisDeployerInterface, error)
}

// kurtosisDeployerInterface describes a subset of functionality required from KurtosisDeployer struct
type kurtosisDeployerInterface interface {
	GetEnvironmentInfo(ctx context.Context, spec *spec.EnclaveSpec) (*kurtosis.KurtosisEnvironment, error)
}

// defaultKurtosisImpl implements kurtosisInterface
type defaultKurtosisImpl struct{}

func (d *defaultKurtosisImpl) NewKurtosisDeployer(opts ...kurtosis.KurtosisDeployerOptions) (kurtosisDeployerInterface, error) {
	return kurtosis.NewKurtosisDeployer(opts...)
}
