package kurtosis

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis/sources/inspect"
)

// ServiceFinder is the main entry point for finding service endpoints
type ServiceFinder struct {
	services         inspect.ServiceMap
	nodeServices     []string
	interestingPorts []string
	l2ServicePrefix  string
}

// ServiceFinderOption configures a ServiceFinder
type ServiceFinderOption func(*ServiceFinder)

// WithNodeServices sets the node service identifiers
func WithNodeServices(services []string) ServiceFinderOption {
	return func(f *ServiceFinder) {
		f.nodeServices = services
	}
}

// WithInterestingPorts sets the ports to look for
func WithInterestingPorts(ports []string) ServiceFinderOption {
	return func(f *ServiceFinder) {
		f.interestingPorts = ports
	}
}

// WithL2ServicePrefix sets the prefix used to identify L2 services
func WithL2ServicePrefix(prefix string) ServiceFinderOption {
	return func(f *ServiceFinder) {
		f.l2ServicePrefix = prefix
	}
}

// NewServiceFinder creates a new ServiceFinder with the given options
func NewServiceFinder(services inspect.ServiceMap, opts ...ServiceFinderOption) *ServiceFinder {
	f := &ServiceFinder{
		services:         services,
		nodeServices:     []string{"cl", "el"},
		interestingPorts: []string{"rpc", "http"},
		l2ServicePrefix:  "op-",
	}
	for _, opt := range opts {
		opt(f)
	}
	return f
}

// FindL1Endpoints finds L1 nodes. Currently returns empty endpoints as specified.
func (f *ServiceFinder) FindL1Endpoints() ([]Node, EndpointMap) {
	return f.findRPCEndpoints(func(serviceName string) (string, int, bool) {
		// Only match services that start with one of the node service identifiers.
		// We might have to change this if we need to support L1 services beyond nodes.
		for _, service := range f.nodeServices {
			if strings.HasPrefix(serviceName, service) {
				tag, idx := f.serviceTag(serviceName)
				return tag, idx, true
			}
		}
		return "", 0, false
	})
}

// FindL2Endpoints finds L2 nodes and endpoints for a specific network
func (f *ServiceFinder) FindL2Endpoints(network string) ([]Node, EndpointMap) {
	networkSuffix := "-" + network
	return f.findRPCEndpoints(func(serviceName string) (string, int, bool) {
		if strings.HasSuffix(serviceName, networkSuffix) {
			name := strings.TrimSuffix(serviceName, networkSuffix)
			tag, idx := f.serviceTag(strings.TrimPrefix(name, f.l2ServicePrefix))
			return tag, idx, true
		}
		return "", 0, false
	})
}

// findRPCEndpoints looks for services matching the given predicate that have an RPC port
func (f *ServiceFinder) findRPCEndpoints(matchService func(string) (string, int, bool)) ([]Node, EndpointMap) {
	endpointMap := make(EndpointMap)
	var nodes []Node

	for serviceName, ports := range f.services {
		var portInfo *inspect.PortInfo
		for _, interestingPort := range f.interestingPorts {
			if p, ok := ports[interestingPort]; ok {
				portInfo = &p
				break
			}
		}
		if portInfo == nil {
			continue
		}

		if serviceIdentifier, num, ok := matchService(serviceName); ok {
			var allocated bool
			for _, service := range f.nodeServices {
				if serviceIdentifier == service {
					if num > len(nodes) {
						nodes = append(nodes, make(Node))
					}
					host := portInfo.Host
					if host == "" {
						host = "localhost"
					}
					nodes[num-1][serviceIdentifier] = fmt.Sprintf("http://%s:%d", host, portInfo.Port)
					allocated = true
				}
			}
			if !allocated {
				host := portInfo.Host
				if host == "" {
					host = "localhost"
				}
				endpointMap[serviceIdentifier] = fmt.Sprintf("http://%s:%d", host, portInfo.Port)
			}
		}
	}
	return nodes, endpointMap
}

// serviceTag returns the shorthand service tag and index if it's a service with multiple instances
func (f *ServiceFinder) serviceTag(serviceName string) (string, int) {
	// Find start of number sequence
	start := strings.IndexFunc(serviceName, func(r rune) bool {
		return r >= '0' && r <= '9'
	})
	if start == -1 {
		return serviceName, 0
	}

	// Find end of number sequence
	end := start + 1
	for end < len(serviceName) && serviceName[end] >= '0' && serviceName[end] <= '9' {
		end++
	}

	idx, err := strconv.Atoi(serviceName[start:end])
	if err != nil {
		return serviceName, 0
	}
	return serviceName[:start-1], idx
}
