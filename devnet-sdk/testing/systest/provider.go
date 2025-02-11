package systest

import "github.com/ethereum-optimism/optimism/devnet-sdk/system"

// systemProvider defines the interface for package-level functionality
type systemProvider interface {
	NewSystemFromEnv(string) (system.System, error)
}

// defaultProvider is the default implementation of the package
type defaultProvider struct{}

func (p *defaultProvider) NewSystemFromEnv(envVar string) (system.System, error) {
	return system.NewSystemFromEnv(envVar)
}

// currentPackage is the current package implementation
var currentPackage systemProvider = &defaultProvider{}
