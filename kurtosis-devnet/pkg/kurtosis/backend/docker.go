package backend

import (
	"net"
	"net/url"
	"strings"
)

// IPNetAddr is an interface that allows getting the underlying *net.IPNet
type IPNetAddr interface {
	net.Addr
	AsIPNet() *net.IPNet
}

// DockerFlavor interface and implementations
type DockerFlavor interface {
	GetDockerHost() string
}

type DockerDesktop struct{}

func (d *DockerDesktop) GetDockerHost() string {
	return "host.docker.internal"
}

type DockerVM struct {
	ipAddress       string
	networkProvider networkProvider
}

func NewDockerVM(ipAddress string) *DockerVM {
	return &DockerVM{
		ipAddress:       ipAddress,
		networkProvider: defaultNetworkProvider{},
	}
}

func (d *DockerVM) GetDockerHost() string {
	vmIP := net.ParseIP(d.ipAddress)
	if vmIP == nil {
		return "localhost"
	}

	ifaces, err := d.networkProvider.Interfaces()
	if err != nil {
		return "localhost"
	}

	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil || len(addrs) == 0 {
			continue
		}

		for _, addr := range addrs {
			ipNetAddr, ok := addr.(IPNetAddr)
			if !ok {
				continue
			}
			ipNet := ipNetAddr.AsIPNet()

			// Skip loopback addresses
			if ipNet.IP.IsLoopback() {
				continue
			}

			// Check if this network contains the VM IP
			if ipNet.Contains(vmIP) {
				// Return our IP address on this interface
				if localIP := ipNet.IP.To4(); localIP != nil {
					return localIP.String()
				}
			}
		}
	}

	return "localhost"
}

type DockerLocal struct {
	networkProvider networkProvider
}

func NewDockerLocal() *DockerLocal {
	return &DockerLocal{
		networkProvider: defaultNetworkProvider{},
	}
}

func (d *DockerLocal) GetDockerHost() string {
	ifaces, err := d.networkProvider.Interfaces()
	if err != nil {
		return "localhost"
	}

	for _, iface := range ifaces {
		if strings.HasPrefix(iface.Name(), "docker") {
			addrs, err := iface.Addrs()
			if err != nil || len(addrs) == 0 {
				continue
			}
			// Get the first IP address
			ipNetAddr, ok := addrs[0].(IPNetAddr)
			if !ok {
				continue
			}
			ipNet := ipNetAddr.AsIPNet()
			return ipNet.IP.String()
		}
	}

	return "localhost"
}

type DockerDetector struct {
	envProvider     envProvider
	runtimeProvider runtimeProvider
}

func NewDockerDetector() *DockerDetector {
	return &DockerDetector{
		envProvider:     defaultEnvProvider{},
		runtimeProvider: defaultRuntimeProvider{},
	}
}

func (d *DockerDetector) DockerFlavor() (DockerFlavor, error) {
	// Check DOCKER_HOST environment variable first as it takes precedence
	if dockerHost := d.envProvider.Getenv("DOCKER_HOST"); dockerHost != "" {
		parsedURL, err := url.Parse(dockerHost)
		if err != nil {
			return nil, err
		}

		if d.runtimeProvider.GOOS() == "linux" && parsedURL.Scheme == "unix" {
			return NewDockerLocal(), nil
		}

		if parsedURL.Scheme == "tcp" {
			return NewDockerVM(parsedURL.Hostname()), nil
		}
	}

	if d.runtimeProvider.GOOS() == "darwin" || d.runtimeProvider.GOOS() == "windows" {
		// TODO: Add actual Docker version check here when needed
		// For now, assume Docker Desktop as it's the most common case
		return &DockerDesktop{}, nil
	}

	// On Linux, default to DockerLocal
	return NewDockerLocal(), nil
}

func DefaultDockerHost() string {
	detector := NewDockerDetector()
	flavor, err := detector.DockerFlavor()
	if err != nil {
		return "localhost"
	}
	return flavor.GetDockerHost()
}
