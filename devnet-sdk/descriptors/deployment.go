package descriptors

import "github.com/ethereum-optimism/optimism/devnet-sdk/types"

type PortInfo struct {
	Host        string `json:"host"`
	Port        int    `json:"port,omitempty"`
	PrivatePort int    `json:"private_port,omitempty"`
}

// EndpointMap is a map of service names to their endpoints.
type EndpointMap map[string]PortInfo

// Service represents a chain service (e.g. batcher, proposer, challenger)
type Service struct {
	Name      string      `json:"name"`
	Endpoints EndpointMap `json:"endpoints"`
}

// ServiceMap is a map of service names to services.
type ServiceMap map[string]Service

// Node represents a node for a chain
type Node struct {
	Services ServiceMap `json:"services"`
}

// AddressMap is a map of address names to their corresponding addresses
type AddressMap map[string]types.Address

// Chain represents a chain (L1 or L2) in a devnet.
type Chain struct {
	Name      string     `json:"name"`
	ID        string     `json:"id,omitempty"`
	Services  ServiceMap `json:"services,omitempty"`
	Nodes     []Node     `json:"nodes"`
	Addresses AddressMap `json:"addresses,omitempty"`
	Wallets   WalletMap  `json:"wallets,omitempty"`
	JWT       string     `json:"jwt,omitempty"`
}

// Wallet represents a wallet with an address and optional private key.
type Wallet struct {
	Address    types.Address `json:"address"`
	PrivateKey string        `json:"private_key,omitempty"`
}

// WalletMap is a map of wallet names to wallets.
type WalletMap map[string]Wallet

// DevnetEnvironment exposes the relevant information to interact with a devnet.
type DevnetEnvironment struct {
	L1 *Chain   `json:"l1"`
	L2 []*Chain `json:"l2"`

	Features []string `json:"features,omitempty"`
}
