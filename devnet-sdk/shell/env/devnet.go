package env

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/ethereum-optimism/optimism/devnet-sdk/descriptors"
)

type DevnetEnv struct {
	Config descriptors.DevnetEnvironment
	Name   string
	URL    string
}

// DataFetcher is a function type for fetching data from a URL
type DataFetcher func(*url.URL) (string, []byte, error)

// schemeToFetcher maps URL schemes to their respective data fetcher functions
var schemeToFetcher = map[string]DataFetcher{
	"":     fetchFileData,
	"file": fetchFileData,
}

// fetchDevnetData retrieves data from a URL based on its scheme
func fetchDevnetData(devnetURL string) (string, []byte, error) {
	parsedURL, err := url.Parse(devnetURL)
	if err != nil {
		return "", nil, fmt.Errorf("error parsing URL: %w", err)
	}

	scheme := strings.ToLower(parsedURL.Scheme)
	fetcher, ok := schemeToFetcher[scheme]
	if !ok {
		return "", nil, fmt.Errorf("unsupported URL scheme: %s", scheme)
	}

	return fetcher(parsedURL)
}

// fetchFileData reads data from a local file
func fetchFileData(u *url.URL) (string, []byte, error) {
	body, err := os.ReadFile(u.Path)
	if err != nil {
		return "", nil, fmt.Errorf("error reading file: %w", err)
	}

	basename := u.Path
	if lastSlash := strings.LastIndex(basename, "/"); lastSlash >= 0 {
		basename = basename[lastSlash+1:]
	}
	if lastDot := strings.LastIndex(basename, "."); lastDot >= 0 {
		basename = basename[:lastDot]
	}
	return basename, body, nil
}

func LoadDevnetFromURL(devnetURL string) (*DevnetEnv, error) {
	name, data, err := fetchDevnetData(devnetURL)
	if err != nil {
		return nil, fmt.Errorf("error fetching devnet data: %w", err)
	}

	var config descriptors.DevnetEnvironment
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("error parsing JSON: %w", err)
	}

	return &DevnetEnv{
		Config: config,
		Name:   name,
		URL:    devnetURL,
	}, nil
}

func (d *DevnetEnv) GetChain(chainName string) (*ChainConfig, error) {
	var chain *descriptors.Chain
	if d.Config.L1.Name == chainName {
		chain = d.Config.L1
	} else {
		for _, l2Chain := range d.Config.L2 {
			if l2Chain.Name == chainName {
				chain = l2Chain
				break
			}
		}
	}

	if chain == nil {
		return nil, fmt.Errorf("chain '%s' not found in devnet config", chainName)
	}

	return &ChainConfig{
		chain:     chain,
		devnetURL: d.URL,
		name:      chainName,
	}, nil
}
