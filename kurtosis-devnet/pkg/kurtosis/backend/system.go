package backend

import (
	"net"
	"os"
	"runtime"
)

// Interface represents a network interface
type Interface interface {
	Addrs() ([]net.Addr, error)
	Name() string
}

// Providers for external dependencies
type networkProvider interface {
	Interfaces() ([]Interface, error)
}

type envProvider interface {
	Getenv(key string) string
}

type runtimeProvider interface {
	GOOS() string
}

// netInterfaceWrapper wraps net.Interface to implement our Interface
type netInterfaceWrapper struct {
	iface net.Interface
}

func (n *netInterfaceWrapper) Addrs() ([]net.Addr, error) {
	addrs, err := n.iface.Addrs()
	if err != nil {
		return nil, err
	}
	// Wrap each address in our IPNetAddr interface if it's an *net.IPNet
	result := make([]net.Addr, len(addrs))
	for i, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok {
			result[i] = &realIPNetAddr{addr: ipNet}
		} else {
			result[i] = addr
		}
	}
	return result, nil
}

func (n *netInterfaceWrapper) Name() string {
	return n.iface.Name
}

// realIPNetAddr wraps a real *net.IPNet to implement our IPNetAddr interface
type realIPNetAddr struct {
	addr *net.IPNet
}

func (r *realIPNetAddr) Network() string { return r.addr.Network() }
func (r *realIPNetAddr) String() string  { return r.addr.String() }
func (r *realIPNetAddr) AsIPNet() *net.IPNet {
	return r.addr
}

// Default implementations
type defaultNetworkProvider struct{}

func (d defaultNetworkProvider) Interfaces() ([]Interface, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	result := make([]Interface, len(ifaces))
	for i, iface := range ifaces {
		// Need to create a new variable here to avoid having all wrappers
		// point to the last interface in the loop
		iface := iface
		result[i] = &netInterfaceWrapper{iface: iface}
	}
	return result, nil
}

type defaultEnvProvider struct{}

func (d defaultEnvProvider) Getenv(key string) string {
	return os.Getenv(key)
}

type defaultRuntimeProvider struct{}

func (d defaultRuntimeProvider) GOOS() string {
	return runtime.GOOS
}
