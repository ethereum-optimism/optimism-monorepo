package systest

import "github.com/ethereum-optimism/optimism/devnet-sdk/system"

// systemProvider defines the interface for package-level functionality
type systemProvider interface {
	NewSystemFromURL(string) (system.System, error)
}

// defaultProvider is the default implementation of the package
type defaultProvider struct{}

func (p *defaultProvider) NewSystemFromURL(url string) (system.System, error) {
	return system.NewSystemFromURL(url)
}

// currentPackage is the current package implementation
var currentPackage systemProvider = &defaultProvider{}
