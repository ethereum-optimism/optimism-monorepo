package kt

import (
	"testing"

	"github.com/ethereum-optimism/optimism/devnet-sdk/manifest"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestKurtosisVisitor_TransformsManifest(t *testing.T) {
	input := `
name: alpaca
type: alphanet
l1:
  name: sepolia
  chain_id: 11155111
l2:
  deployment:
    op-deployer:
      version: op-deployer/v0.0.11
    l1-contracts:
      locator: https://storage.googleapis.com/oplabs-contract-artifacts/artifacts-v1-c3f2e2adbd52a93c2c08cab018cd637a4e203db53034e59c6c139c76b4297953.tar.gz
      version: 984bae9146398a2997ec13757bfe2438ca8f92eb
    l2-contracts:
      version: op-contracts/v1.7.0-beta.1+l2-contracts
    overrides:
      seconds_per_slot: 2
      fjord_time_offset: 0
      granite_time_offset: 0
      holocene_time_offset: 0
  components:
    op-node:
      version: op-node/v1.10.2
    op-geth:
      version: op-geth/v1.101411.4-rc.4
    op-reth:
      version: op-reth/v1.1.5
    op-proposer:
      version: op-proposer/v1.10.0-rc.2
    op-batcher:
      version: op-batcher/v1.10.0
    op-challenger:
      version: op-challenger/v1.3.1-rc.4
  chains:
  - name: alpaca-0
    chain_id: 11155111100000
`

	// Then the output should match the expected YAML structure
	expected := KurtosisParams{
		OptimismPackage: OptimismPackage{
			Chains: []ChainConfig{
				{
					Participants: []ParticipantConfig{
						{
							ElType:  "op-geth",
							ElImage: "us-docker.pkg.dev/oplabs-tools-artifacts/images/op-geth:v1.101411.4-rc.4",
							ClType:  "op-node",
							ClImage: "us-docker.pkg.dev/oplabs-tools-artifacts/images/op-node:v1.10.2",
							Count:   1,
						},
					},
					NetworkParams: NetworkParams{
						Network:         "kurtosis",
						NetworkID:       "1081032288",
						SecondsPerSlot:  2,
						Name:            "alpaca-0",
						FundDevAccounts: true,
						TimeOffsets: TimeOffsets{
							"fjord_time_offset":    0,
							"granite_time_offset":  0,
							"holocene_time_offset": 0,
						},
					},
					BatcherParams: BatcherParams{
						Image: "us-docker.pkg.dev/oplabs-tools-artifacts/images/op-batcher:v1.10.0",
					},
					ChallengerParams: ChallengerParams{
						Image:              "us-docker.pkg.dev/oplabs-tools-artifacts/images/op-challenger:v1.3.1-rc.4",
						CannonPrestatesURL: "",
					},
					ProposerParams: ProposerParams{
						Image:            "us-docker.pkg.dev/oplabs-tools-artifacts/images/op-proposer:v1.10.0-rc.2",
						GameType:         1,
						ProposalInterval: "10m",
					},
				},
			},
			OpContractDeployerParams: OpContractDeployerParams{
				Image:              "us-docker.pkg.dev/oplabs-tools-artifacts/images/op-deployer:v0.0.11",
				L1ArtifactsLocator: "https://storage.googleapis.com/oplabs-contract-artifacts/artifacts-v1-c3f2e2adbd52a93c2c08cab018cd637a4e203db53034e59c6c139c76b4297953.tar.gz",
				L2ArtifactsLocator: "tag://op-contracts/v1.7.0-beta.1+l2-contracts",
			},
			Persistent: false,
		},
		EthereumPackage: EthereumPackage{
			NetworkParams: EthereumNetworkParams{
				Preset:                       "minimal",
				GenesisDelay:                 5,
				AdditionalPreloadedContracts: defaultPreloadedContracts,
			},
		},
	}

	// Convert the input to a manifest
	var manifest manifest.Manifest
	err := yaml.Unmarshal([]byte(input), &manifest)
	require.NoError(t, err)

	// Create visitor and have manifest accept it
	visitor := NewKurtosisVisitor()
	manifest.Accept(visitor)

	// Get the generated params
	actual := *visitor.GetParams()

	// Compare the actual and expected params
	require.Equal(t, expected, actual, "Generated params should match expected params")

}
