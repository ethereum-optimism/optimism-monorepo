package env

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis"
	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis/sources/spec"
)

// parseKurtosisURL parses a Kurtosis URL of the form kt://enclave/artifact/file
// If artifact is omitted, it defaults to "devnet"
// If file is omitted, it defaults to "env.json"
func parseKurtosisNativeURL(u *url.URL) (enclave, argsFileName string) {
	enclave = u.Host
	argsFileName = "/" + strings.Trim(u.Path, "/")

	return
}

// fetchKurtosisData reads data from a Kurtosis artifact
func fetchKurtosisNativeData(u *url.URL) (string, []byte, error) {
	// First let's parse the kurtosis URL
	enclave, argsFileName := parseKurtosisNativeURL(u)

	// Open the arguments file
	argsFile, err := os.Open(argsFileName)
	if err != nil {
		return "", nil, fmt.Errorf("error reading arguments file: %w", err)
	}

	// Make sure to close the file once we're done reading
	defer argsFile.Close()

	// Once we have the arguments file, we can extract the enclave spec
	enclaveSpec, err := spec.NewSpec().ExtractData(argsFile)
	if err != nil {
		return enclave, nil, fmt.Errorf("error extracting enclave spec: %w", err)
	}

	// We'll use the deployer to extract the system spec
	deployer, err := kurtosis.NewKurtosisDeployer(kurtosis.WithKurtosisEnclave(enclave))
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
