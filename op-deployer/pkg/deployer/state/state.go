package state

import (
	"fmt"

	"github.com/ethereum/go-ethereum/core/types"

	"github.com/ethereum-optimism/optimism/op-chain-ops/foundry"
	"github.com/ethereum-optimism/optimism/op-service/ioutil"
	"github.com/ethereum-optimism/optimism/op-service/jsonutil"
	"github.com/ethereum/go-ethereum/common"
)

// State contains the data needed to recreate the deployment
// as it progresses and once it is fully applied.
type State struct {
	// Version versions the state so we can update it later.
	Version int `json:"version"`

	// Create2Salt is the salt used for CREATE2 deployments.
	Create2Salt common.Hash `json:"create2Salt"`

	// AppliedIntent contains the chain intent that was last
	// successfully applied. It is diffed against new intent
	// in order to determine what deployment steps to take.
	// This field is nil for new deployments.
	AppliedIntent *Intent `json:"appliedIntent"`

	// SuperchainDeployment contains the addresses of the Superchain
	// deployment. It only contains the proxies because the implementations
	// can be looked up on chain.
	SuperchainDeployment *SuperchainDeployment `json:"superchainDeployment"`

	// ImplementationsDeployment contains the addresses of the common implementation
	// contracts required for the Superchain to function.
	ImplementationsDeployment *ImplementationsDeployment `json:"implementationsDeployment"`

	// Chains contains data about L2 chain deployments.
	Chains []*ChainState `json:"opChainDeployments"`

	// L1StateDump contains the complete L1 state dump of the deployment.
	L1StateDump *GzipData[foundry.ForgeAllocs] `json:"l1StateDump"`
}

func (s *State) WriteToFile(path string) error {
	return jsonutil.WriteJSON(s, ioutil.ToAtomicFile(path, 0o755))
}

func (s *State) Chain(id common.Hash) (*ChainState, error) {
	for _, chain := range s.Chains {
		if chain.ID == id {
			return chain, nil
		}
	}
	return nil, fmt.Errorf("chain not found: %s", id.Hex())
}

type SuperchainDeployment struct {
	ProxyAdminAddress            common.Address `json:"proxyAdminAddress"`
	SuperchainConfigProxyAddress common.Address `json:"superchainConfigProxyAddress"`
	SuperchainConfigImplAddress  common.Address `json:"superchainConfigImplAddress"`
	ProtocolVersionsProxyAddress common.Address `json:"protocolVersionsProxyAddress"`
	ProtocolVersionsImplAddress  common.Address `json:"protocolVersionsImplAddress"`
}

type ImplementationsDeployment struct {
	OpcmAddress                             common.Address `json:"opcmAddress"`
	DelayedWETHImplAddress                  common.Address `json:"delayedWETHImplAddress"`
	OptimismPortalImplAddress               common.Address `json:"optimismPortalImplAddress"`
	PreimageOracleSingletonAddress          common.Address `json:"preimageOracleSingletonAddress"`
	MipsSingletonAddress                    common.Address `json:"mipsSingletonAddress"`
	SystemConfigImplAddress                 common.Address `json:"systemConfigImplAddress"`
	L1CrossDomainMessengerImplAddress       common.Address `json:"l1CrossDomainMessengerImplAddress"`
	L1ERC721BridgeImplAddress               common.Address `json:"l1ERC721BridgeImplAddress"`
	L1StandardBridgeImplAddress             common.Address `json:"l1StandardBridgeImplAddress"`
	OptimismMintableERC20FactoryImplAddress common.Address `json:"optimismMintableERC20FactoryImplAddress"`
	DisputeGameFactoryImplAddress           common.Address `json:"disputeGameFactoryImplAddress"`
}

type ChainState struct {
	ID common.Hash `json:"id"`

	ProxyAdminAddress                         common.Address `json:"proxyAdminAddress"`
	AddressManagerAddress                     common.Address `json:"addressManagerAddress"`
	L1ERC721BridgeProxyAddress                common.Address `json:"l1ERC721BridgeProxyAddress"`
	SystemConfigProxyAddress                  common.Address `json:"systemConfigProxyAddress"`
	OptimismMintableERC20FactoryProxyAddress  common.Address `json:"optimismMintableERC20FactoryProxyAddress"`
	L1StandardBridgeProxyAddress              common.Address `json:"l1StandardBridgeProxyAddress"`
	L1CrossDomainMessengerProxyAddress        common.Address `json:"l1CrossDomainMessengerProxyAddress"`
	OptimismPortalProxyAddress                common.Address `json:"optimismPortalProxyAddress"`
	DisputeGameFactoryProxyAddress            common.Address `json:"disputeGameFactoryProxyAddress"`
	AnchorStateRegistryProxyAddress           common.Address `json:"anchorStateRegistryProxyAddress"`
	AnchorStateRegistryImplAddress            common.Address `json:"anchorStateRegistryImplAddress"`
	FaultDisputeGameAddress                   common.Address `json:"faultDisputeGameAddress"`
	PermissionedDisputeGameAddress            common.Address `json:"permissionedDisputeGameAddress"`
	DelayedWETHPermissionedGameProxyAddress   common.Address `json:"delayedWETHPermissionedGameProxyAddress"`
	DelayedWETHPermissionlessGameProxyAddress common.Address `json:"delayedWETHPermissionlessGameProxyAddress"`
	DataAvailabilityChallengeProxyAddress     common.Address `json:"dataAvailabilityChallengeProxyAddress"`
	DataAvailabilityChallengeImplAddress      common.Address `json:"dataAvailabilityChallengeImplAddress"`

	Allocs *GzipData[foundry.ForgeAllocs] `json:"allocs"`

	StartBlock *types.Header `json:"startBlock"`
}
