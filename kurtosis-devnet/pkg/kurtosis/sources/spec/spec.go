package spec

import (
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

// ChainSpec represents the network parameters for a chain
type ChainSpec struct {
	Name      string
	NetworkID string
}

// EnclaveSpec represents the parsed chain specifications from the YAML
type EnclaveSpec struct {
	Chains   []ChainSpec
	Features []string
}

// NetworkParams represents the network parameters section in the YAML
type NetworkParams struct {
	Name      string `yaml:"name"`
	NetworkID string `yaml:"network_id"`
}

// ChainConfig represents a chain configuration in the YAML
type ChainConfig struct {
	NetworkParams NetworkParams `yaml:"network_params"`
}

// InteropConfig represents the interop section in the YAML
type InteropConfig struct {
	Enabled bool `yaml:"enabled"`
}

// OptimismPackage represents the optimism_package section in the YAML
type OptimismPackage struct {
	Interop InteropConfig `yaml:"interop"`
	Chains  []ChainConfig `yaml:"chains"`
}

// YAMLSpec represents the root of the YAML document
type YAMLSpec struct {
	OptimismPackage OptimismPackage `yaml:"optimism_package"`
}

type Spec struct{}

type SpecOption func(*Spec)

func NewSpec(opts ...SpecOption) *Spec {
	s := &Spec{}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

type featureExtractor func(YAMLSpec, string) bool

var featuresMap = map[string]featureExtractor{
	"interop": interopExtractor,
}

func interopExtractor(yamlSpec YAMLSpec, chainName string) bool {
	return yamlSpec.OptimismPackage.Interop.Enabled
}

// ExtractData parses a YAML document and returns the chain specifications
func (s *Spec) ExtractData(r io.Reader) (*EnclaveSpec, error) {
	var yamlSpec YAMLSpec
	decoder := yaml.NewDecoder(r)
	if err := decoder.Decode(&yamlSpec); err != nil {
		return nil, fmt.Errorf("failed to decode YAML: %w", err)
	}

	var features []string
	for feature, extractor := range featuresMap {
		if extractor(yamlSpec, feature) {
			features = append(features, feature)
		}
	}

	result := &EnclaveSpec{
		Chains:   make([]ChainSpec, 0, len(yamlSpec.OptimismPackage.Chains)),
		Features: features,
	}

	// Extract chain specifications
	for _, chain := range yamlSpec.OptimismPackage.Chains {
		result.Chains = append(result.Chains, ChainSpec{
			Name:      chain.NetworkParams.Name,
			NetworkID: chain.NetworkParams.NetworkID,
		})
	}

	return result, nil
}
