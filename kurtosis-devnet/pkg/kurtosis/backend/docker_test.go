package backend

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock implementations
type mockInterface struct {
	name  string
	addrs []net.Addr
}

func (m *mockInterface) Name() string {
	return m.name
}

func (m *mockInterface) Addrs() ([]net.Addr, error) {
	return m.addrs, nil
}

type mockNetworkProvider struct {
	interfaces []Interface
}

func (m *mockNetworkProvider) Interfaces() ([]Interface, error) {
	return m.interfaces, nil
}

type mockEnvProvider struct {
	env map[string]string
}

func (m *mockEnvProvider) Getenv(key string) string {
	return m.env[key]
}

type mockRuntimeProvider struct {
	goos string
}

func (m *mockRuntimeProvider) GOOS() string {
	return m.goos
}

// mockIPNet implements net.Addr and can be type asserted to *net.IPNet
type mockIPNet struct {
	addr *net.IPNet
}

func newMockIPNet(cidr string) net.Addr {
	ip, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		panic(err)
	}
	// Set the IP in the IPNet to be the specific IP, not the network address
	ipNet.IP = ip
	return &mockIPNet{addr: ipNet}
}

func (m *mockIPNet) Network() string { return "ip+net" }
func (m *mockIPNet) String() string  { return m.addr.IP.String() }

// Make it possible to type assert to *net.IPNet
func (m *mockIPNet) AsIPNet() *net.IPNet {
	return m.addr
}

func TestDockerDesktop(t *testing.T) {
	flavor := &DockerDesktop{}
	assert.Equal(t, "host.docker.internal", flavor.GetDockerHost())
}

func TestDockerVM(t *testing.T) {
	tests := []struct {
		name     string
		vmIP     string
		ifaces   []Interface
		expected string
	}{
		{
			name: "matching interface found",
			vmIP: "192.168.1.100",
			ifaces: []Interface{
				&mockInterface{
					name:  "eth0",
					addrs: []net.Addr{newMockIPNet("192.168.1.5/24")},
				},
			},
			expected: "192.168.1.5",
		},
		{
			name: "no matching interface",
			vmIP: "10.0.0.100",
			ifaces: []Interface{
				&mockInterface{
					name:  "eth0",
					addrs: []net.Addr{newMockIPNet("192.168.1.5/24")},
				},
			},
			expected: "localhost",
		},
		{
			name:     "invalid VM IP",
			vmIP:     "invalid-ip",
			ifaces:   []Interface{},
			expected: "localhost",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vm := NewDockerVM(tt.vmIP)
			vm.networkProvider = &mockNetworkProvider{interfaces: tt.ifaces}
			assert.Equal(t, tt.expected, vm.GetDockerHost())
		})
	}
}

func TestDockerLocal(t *testing.T) {
	tests := []struct {
		name     string
		ifaces   []Interface
		expected string
	}{
		{
			name: "docker0 interface found",
			ifaces: []Interface{
				&mockInterface{
					name:  "docker0",
					addrs: []net.Addr{newMockIPNet("172.17.0.1/16")},
				},
			},
			expected: "172.17.0.1",
		},
		{
			name: "docker1 interface found",
			ifaces: []Interface{
				&mockInterface{
					name:  "docker1",
					addrs: []net.Addr{newMockIPNet("172.18.0.1/16")},
				},
			},
			expected: "172.18.0.1",
		},
		{
			name: "prefers first docker interface",
			ifaces: []Interface{
				&mockInterface{
					name:  "eth0",
					addrs: []net.Addr{newMockIPNet("192.168.1.5/24")},
				},
				&mockInterface{
					name:  "docker0",
					addrs: []net.Addr{newMockIPNet("172.17.0.1/16")},
				},
				&mockInterface{
					name:  "docker1",
					addrs: []net.Addr{newMockIPNet("172.18.0.1/16")},
				},
			},
			expected: "172.17.0.1",
		},
		{
			name: "skips docker interface with no addresses",
			ifaces: []Interface{
				&mockInterface{
					name:  "docker0",
					addrs: []net.Addr{},
				},
				&mockInterface{
					name:  "docker1",
					addrs: []net.Addr{newMockIPNet("172.18.0.1/16")},
				},
			},
			expected: "172.18.0.1",
		},
		{
			name: "no docker interface",
			ifaces: []Interface{
				&mockInterface{
					name:  "eth0",
					addrs: []net.Addr{newMockIPNet("192.168.1.5/24")},
				},
			},
			expected: "localhost",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			local := NewDockerLocal()
			local.networkProvider = &mockNetworkProvider{interfaces: tt.ifaces}
			assert.Equal(t, tt.expected, local.GetDockerHost())
		})
	}
}

func TestDockerDetector(t *testing.T) {
	tests := []struct {
		name        string
		env         map[string]string
		goos        string
		expectType  string
		expectError bool
	}{
		{
			name: "unix socket",
			env: map[string]string{
				"DOCKER_HOST": "unix:///var/run/docker.sock",
			},
			expectType: "DockerLocal",
		},
		{
			name: "tcp host",
			env: map[string]string{
				"DOCKER_HOST": "tcp://192.168.1.100:2375",
			},
			expectType: "DockerVM",
		},
		{
			name:       "darwin no docker host",
			env:        map[string]string{},
			goos:       "darwin",
			expectType: "DockerDesktop",
		},
		{
			name:       "windows no docker host",
			env:        map[string]string{},
			goos:       "windows",
			expectType: "DockerDesktop",
		},
		{
			name:       "linux no docker host",
			env:        map[string]string{},
			goos:       "linux",
			expectType: "DockerLocal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detector := NewDockerDetector()
			detector.envProvider = &mockEnvProvider{env: tt.env}
			detector.runtimeProvider = &mockRuntimeProvider{goos: tt.goos}

			flavor, err := detector.DockerFlavor()
			if tt.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			var gotType string
			switch flavor.(type) {
			case *DockerLocal:
				gotType = "DockerLocal"
			case *DockerVM:
				gotType = "DockerVM"
			case *DockerDesktop:
				gotType = "DockerDesktop"
			}

			assert.Equal(t, tt.expectType, gotType)
		})
	}
}
