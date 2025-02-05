package kt

import (
	"strconv"
	"strings"

	"github.com/ethereum-optimism/optimism/devnet-sdk/images"
	"github.com/ethereum-optimism/optimism/devnet-sdk/manifest"
)

const (
	defaultProposalInterval   = "10m"
	defaultGameType           = 1
	defaultPreset             = "minimal"
	defaultGenesisDelay       = 5
	defaultPreloadedContracts = `{
            "0x4e59b44847b379578588920cA78FbF26c0B4956C": {
                    "balance": "0ETH",
                    "code": "0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe03601600081602082378035828234f58015156039578182fd5b8082525050506014600cf3",
                    "storage": {},
                    "nonce": "1"
            }
    }`
)

// KurtosisVisitor implements the manifest.ManifestVisitor interface
type KurtosisVisitor struct {
	params     *KurtosisParams
	repository *images.Repository
	l2Visitor  *l2Visitor
}

// Component visitor for handling component versions
type componentVisitor struct {
	name    string
	version string
}

// Chain visitor for handling chain configuration
type chainVisitor struct {
	name string
	id   uint64
}

// Contracts visitor for handling contract configuration
type contractsVisitor struct {
	locator string
}

// Overrides represents deployment overrides
type Overrides struct {
	SecondsPerSlot int `yaml:"seconds_per_slot"`
	TimeOffsets    `yaml:",inline"`
}

// Deployment visitor for handling deployment configuration
type deploymentVisitor struct {
	deployer    *componentVisitor
	l1Contracts *contractsVisitor
	l2Contracts *contractsVisitor
	overrides   *Overrides
}

// L2 visitor for handling L2 configuration
type l2Visitor struct {
	components map[string]*componentVisitor
	deployment *deploymentVisitor
	chains     []*chainVisitor
}

// NewKurtosisVisitor creates a new KurtosisVisitor
func NewKurtosisVisitor() *KurtosisVisitor {
	return &KurtosisVisitor{
		params: &KurtosisParams{
			OptimismPackage: OptimismPackage{
				Chains:     make([]ChainConfig, 0),
				Persistent: false,
			},
			EthereumPackage: EthereumPackage{
				NetworkParams: EthereumNetworkParams{
					Preset:                       defaultPreset,
					GenesisDelay:                 defaultGenesisDelay,
					AdditionalPreloadedContracts: defaultPreloadedContracts,
				},
			},
		},
		repository: images.NewRepository(),
	}
}

func (v *KurtosisVisitor) VisitName(name string) {}

func (v *KurtosisVisitor) VisitType(manifestType string) {}

func (v *KurtosisVisitor) VisitL1() manifest.ChainVisitor {
	return &chainVisitor{}
}

func (v *KurtosisVisitor) VisitL2() manifest.L2Visitor {
	v.l2Visitor = &l2Visitor{
		components: make(map[string]*componentVisitor),
		deployment: &deploymentVisitor{
			deployer:    &componentVisitor{},
			l1Contracts: &contractsVisitor{},
			l2Contracts: &contractsVisitor{},
			overrides: &Overrides{
				TimeOffsets: make(TimeOffsets),
			},
		},
		chains: make([]*chainVisitor, 0),
	}
	return v.l2Visitor
}

// Component visitor implementation
func (v *componentVisitor) VisitVersion(version string) {
	// Strip the component name from the version string
	parts := strings.SplitN(version, "/", 2)
	if len(parts) == 2 {
		v.version = parts[1]
	} else {
		v.version = version
	}
}

// Chain visitor implementation
func (v *chainVisitor) VisitName(name string) {
	v.name = name
}

func (v *chainVisitor) VisitID(id uint64) {
	// TODO: this is horrible but unfortunately the funding script breaks for
	// chain IDs larger than 32 bits.
	v.id = id & 0xFFFFFFFF
}

// Contracts visitor implementation
func (v *contractsVisitor) VisitVersion(version string) {
	if v.locator == "" {
		v.locator = "tag://" + version
	}
}

func (v *contractsVisitor) VisitLocator(locator string) {
	v.locator = locator
}

// Deployment visitor implementation
func (v *deploymentVisitor) VisitDeployer() manifest.ComponentVisitor {
	return v.deployer
}

func (v *deploymentVisitor) VisitL1Contracts() manifest.ContractsVisitor {
	return v.l1Contracts
}

func (v *deploymentVisitor) VisitL2Contracts() manifest.ContractsVisitor {
	return v.l2Contracts
}

func (v *deploymentVisitor) VisitOverride(key string, value interface{}) {
	if key == "seconds_per_slot" {
		if intValue, ok := value.(int); ok {
			v.overrides.SecondsPerSlot = intValue
		}
	} else if strings.HasSuffix(key, "_time_offset") {
		if intValue, ok := value.(int); ok {
			v.overrides.TimeOffsets[key] = intValue
		}
	}
}

// L2 visitor implementation
func (v *l2Visitor) VisitL2Component(name string) manifest.ComponentVisitor {
	comp := &componentVisitor{name: name}
	v.components[name] = comp
	return comp
}

func (v *l2Visitor) VisitL2Deployment() manifest.DeploymentVisitor {
	return v.deployment
}

func (v *l2Visitor) VisitL2Chain(idx int) manifest.ChainVisitor {
	chain := &chainVisitor{}
	if idx >= len(v.chains) {
		v.chains = append(v.chains, chain)
	} else {
		v.chains[idx] = chain
	}
	return chain
}

// GetParams returns the generated Kurtosis parameters
func (v *KurtosisVisitor) GetParams() *KurtosisParams {
	if v.l2Visitor != nil {
		v.BuildKurtosisParams(v.l2Visitor)
	}
	return v.params
}

// getComponentVersion returns the version for a component, or empty string if not found
func (l2 *l2Visitor) getComponentVersion(name string) string {
	if comp, ok := l2.components[name]; ok {
		return comp.version
	}
	return ""
}

// getComponentImage returns the image for a component, or empty string if component doesn't exist
func (v *KurtosisVisitor) getComponentImage(l2 *l2Visitor, name string) string {
	if _, ok := l2.components[name]; ok {
		return v.repository.GetImage(name, l2.getComponentVersion(name))
	}
	return ""
}

// BuildKurtosisParams builds the final Kurtosis parameters from the collected visitor data
func (v *KurtosisVisitor) BuildKurtosisParams(l2 *l2Visitor) {
	// Set deployer params
	v.params.OptimismPackage.OpContractDeployerParams = OpContractDeployerParams{
		Image:              v.repository.GetImage("op-deployer", l2.deployment.deployer.version),
		L1ArtifactsLocator: l2.deployment.l1Contracts.locator,
		L2ArtifactsLocator: l2.deployment.l2Contracts.locator,
	}

	// Build chain configs
	for _, chain := range l2.chains {
		// Create network params with embedded map
		networkParams := NetworkParams{
			Network:         "kurtosis",
			NetworkID:       strconv.FormatUint(chain.id, 10),
			SecondsPerSlot:  l2.deployment.overrides.SecondsPerSlot,
			Name:            chain.name,
			FundDevAccounts: true,
			TimeOffsets:     l2.deployment.overrides.TimeOffsets,
		}

		chainConfig := ChainConfig{
			Participants: []ParticipantConfig{
				{
					ElType:  "op-geth",
					ElImage: v.getComponentImage(l2, "op-geth"),
					ClType:  "op-node",
					ClImage: v.getComponentImage(l2, "op-node"),
					Count:   1,
				},
			},
			NetworkParams: networkParams,
			BatcherParams: BatcherParams{
				Image: v.getComponentImage(l2, "op-batcher"),
			},
			ChallengerParams: ChallengerParams{
				Image: v.getComponentImage(l2, "op-challenger"),
			},
			ProposerParams: ProposerParams{
				Image:            v.getComponentImage(l2, "op-proposer"),
				GameType:         defaultGameType,
				ProposalInterval: defaultProposalInterval,
			},
		}

		v.params.OptimismPackage.Chains = append(v.params.OptimismPackage.Chains, chainConfig)
	}
}
