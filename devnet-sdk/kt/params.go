package kt

// KurtosisParams represents the top-level Kurtosis configuration
type KurtosisParams struct {
	OptimismPackage OptimismPackage `yaml:"optimism_package"`
	EthereumPackage EthereumPackage `yaml:"ethereum_package"`
}

// OptimismPackage represents the Optimism-specific configuration
type OptimismPackage struct {
	Chains                   []ChainConfig            `yaml:"chains"`
	OpContractDeployerParams OpContractDeployerParams `yaml:"op_contract_deployer_params"`
	Persistent               bool                     `yaml:"persistent"`
}

// ChainConfig represents a single chain configuration
type ChainConfig struct {
	Participants     []ParticipantConfig `yaml:"participants"`
	NetworkParams    NetworkParams       `yaml:"network_params"`
	BatcherParams    BatcherParams       `yaml:"batcher_params"`
	ChallengerParams ChallengerParams    `yaml:"challenger_params"`
	ProposerParams   ProposerParams      `yaml:"proposer_params"`
}

// ParticipantConfig represents a participant in the network
type ParticipantConfig struct {
	ElType  string `yaml:"el_type"`
	ElImage string `yaml:"el_image"`
	ClType  string `yaml:"cl_type"`
	ClImage string `yaml:"cl_image"`
	Count   int    `yaml:"count"`
}

// TimeOffsets represents a map of time offset values
type TimeOffsets map[string]int

// NetworkParams represents network-specific parameters
type NetworkParams struct {
	Network         string `yaml:"network"`
	NetworkID       string `yaml:"network_id"`
	SecondsPerSlot  int    `yaml:"seconds_per_slot"`
	Name            string `yaml:"name"`
	FundDevAccounts bool   `yaml:"fund_dev_accounts"`
	TimeOffsets     `yaml:",inline"`
}

// BatcherParams represents batcher-specific parameters
type BatcherParams struct {
	Image string `yaml:"image"`
}

// ChallengerParams represents challenger-specific parameters
type ChallengerParams struct {
	Image              string `yaml:"image"`
	CannonPrestatesURL string `yaml:"cannon_prestates_url,omitempty"`
}

// ProposerParams represents proposer-specific parameters
type ProposerParams struct {
	Image            string `yaml:"image"`
	GameType         int    `yaml:"game_type"`
	ProposalInterval string `yaml:"proposal_interval"`
}

// OpContractDeployerParams represents contract deployer parameters
type OpContractDeployerParams struct {
	Image              string `yaml:"image"`
	L1ArtifactsLocator string `yaml:"l1_artifacts_locator"`
	L2ArtifactsLocator string `yaml:"l2_artifacts_locator"`
}

// EthereumPackage represents Ethereum-specific configuration
type EthereumPackage struct {
	NetworkParams EthereumNetworkParams `yaml:"network_params"`
}

// EthereumNetworkParams represents Ethereum network parameters
type EthereumNetworkParams struct {
	Preset                       string `yaml:"preset"`
	GenesisDelay                 int    `yaml:"genesis_delay"`
	AdditionalPreloadedContracts string `yaml:"additional_preloaded_contracts"`
}
