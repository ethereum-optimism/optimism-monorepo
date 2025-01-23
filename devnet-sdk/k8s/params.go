package k8s

import (
	"crypto/sha256"
	"encoding/hex"
)

type K8sParams struct {
	Cluster string        `yaml:"cluster"`
	Project string        `yaml:"project"`
	L1Rpc   string        `yaml:"l1_rpc"`
	Chains  []ChainParams `yaml:"chains"`
}

type ChainParams struct {
	ChainName string       `yaml:"chain_name"`
	Nodes     []NodeParams `yaml:"nodes"`
}

type NodeParams struct {
	NodeName     string `yaml:"node_name"`
	NodeType     string `yaml:"node_type"`
	ClUrl        string `yaml:"cl_url"`
	ElUrl        string `yaml:"el_url"`
	ConductorUrl string `yaml:"conductor_url"`
	ClKey        string `yaml:"cl_key"`
	JwtSecret    string `yaml:"jwt_secret"`
}

const (
	DEFAULT_CLUSTER = "oplabs-dev-infra-primary"
	DEFAULT_PROJECT = "oplbads-dev-infra"
	DEFAULT_L1_RPC  = "https://proxyd-l1-sepolia.primary.client.dev.oplabs.cloud"
)

// Generate deterministic hex value from string using SHA256
func generateHexFromString(input string) string {
	hash := sha256.Sum256([]byte(input))
	return hex.EncodeToString(hash[:32])
}
