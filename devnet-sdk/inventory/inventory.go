package inventory

// Inventory represents the top-level inventory structure
type Inventory struct {
	Chains []Chain `yaml:"chains"`
}

// Chain represents a blockchain network configuration
type Chain struct {
	Name     string    `yaml:"name"`
	Nodes    []Node    `yaml:"nodes"`
	Services []Service `yaml:"services"`
}

// Node represents a node in the network
type Node struct {
	Kind string   `yaml:"kind"`
	Name string   `yaml:"name"`
	Spec NodeSpec `yaml:"spec"`
}

// NodeSpec defines the specification for a node
type NodeSpec struct {
	Kind string  `yaml:"kind"`
	EL   Service `yaml:"el"`
	CL   Service `yaml:"cl"`
}

// Service represents a service in the network
type Service struct {
	Kind string      `yaml:"kind"`
	Name string      `yaml:"name"`
	Spec ServiceSpec `yaml:"spec"`
	Deps ServiceDeps `yaml:"deps"`
}

// ServiceSpec defines the specification for a service
type ServiceSpec struct {
	Kind string           `yaml:"kind"`
	Spec ServiceLayerSpec `yaml:"spec"`
}

// ServiceLayerSpec contains the specific configuration for a service
type ServiceLayerSpec struct {
	Version string            `yaml:"version"`
	Env     map[string]string `yaml:"env,omitempty"`
	K8s     map[string]string `yaml:"k8s,omitempty"`
}

// ServiceDeps defines the dependencies for a service
type ServiceDeps struct {
	Nodes []string `yaml:"nodes"`
}
