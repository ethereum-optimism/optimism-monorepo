package env

import (
	"net/url"
	"strings"
)

// parseKurtosisURL parses a Kurtosis URL of the form kt://enclave/artifact/file
// If artifact is omitted, it defaults to "devnet"
// If file is omitted, it defaults to "env.json"
func parseKurtosisNativeURL(u *url.URL) (enclave, argsFileName string) {
	enclave = u.Host
	argsFileName = strings.TrimPrefix(u.Path, "/")

	return
}

// fetchKurtosisData reads data from a Kurtosis artifact
func fetchKurtosisNativeData(u *url.URL) (string, []byte, error) {
	// enclave, argsFileName := parseKurtosisNativeURL(u)

	return "", nil, nil
}
