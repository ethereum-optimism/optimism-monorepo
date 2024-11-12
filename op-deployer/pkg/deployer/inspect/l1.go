package inspect

import (
	"fmt"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/pipeline"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/state"

	"github.com/ethereum-optimism/optimism/op-service/ioutil"
	"github.com/ethereum-optimism/optimism/op-service/jsonutil"
	"github.com/ethereum/go-ethereum/common"
	"github.com/urfave/cli/v2"
)

type L1Contracts struct {
	SuperchainDeployment      SuperchainDeployment      `json:"superchainDeployment"`
	OpChainDeployment         OpChainDeployment         `json:"opChainDeployment"`
	ImplementationsDeployment ImplementationsDeployment `json:"implementationsDeployment"`
}

type SuperchainDeployment struct {
	ProxyAdminAddress            common.Address `json:"proxyAdminAddress"`
	SuperchainConfigProxyAddress common.Address `json:"superchainConfigProxyAddress"`
	SuperchainConfigImplAddress  common.Address `json:"superchainConfigImplAddress"`
	ProtocolVersionsProxyAddress common.Address `json:"protocolVersionsProxyAddress"`
	ProtocolVersionsImplAddress  common.Address `json:"protocolVersionsImplAddress"`
}

type OpChainDeployment struct {
	ProxyAdminAddress                        common.Address `json:"proxyAdminAddress"`
	AddressManagerAddress                    common.Address `json:"addressManagerAddress"`
	L1ERC721BridgeProxyAddress               common.Address `json:"l1ERC721BridgeProxyAddress"`
	SystemConfigProxyAddress                 common.Address `json:"systemConfigProxyAddress"`
	OptimismMintableERC20FactoryProxyAddress common.Address `json:"optimismMintableERC20FactoryProxyAddress"`
	L1StandardBridgeProxyAddress             common.Address `json:"l1StandardBridgeProxyAddress"`
	L1CrossDomainMessengerProxyAddress       common.Address `json:"l1CrossDomainMessengerProxyAddress"`
	OptimismPortalProxyAddress               common.Address `json:"optimismPortalProxyAddress"`
	DisputeGameFactoryProxyAddress           common.Address `json:"disputeGameFactoryProxyAddress"`
	AnchorStateRegistryProxyAddress          common.Address `json:"anchorStateRegistryProxyAddress"`
	AnchorStateRegistryImplAddress           common.Address `json:"anchorStateRegistryImplAddress"`
	FaultDisputeGameAddress                  common.Address `json:"faultDisputeGameAddress"`
	PermissionedDisputeGameAddress           common.Address `json:"permissionedDisputeGameAddress"`
	DelayedWETHPermissionedGameProxyAddress  common.Address `json:"delayedWETHPermissionedGameProxyAddress"`
	// DelayedWETHPermissionlessGameProxyAddress common.Address `json:"delayedWETHPermissionlessGameProxyAddress"`
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

func L1CLI(cliCtx *cli.Context) error {
	cfg, err := readConfig(cliCtx)
	if err != nil {
		return err
	}

	globalState, err := pipeline.ReadState(cfg.Workdir)
	if err != nil {
		return fmt.Errorf("failed to read intent: %w", err)
	}

	l1Contracts, err := L1(globalState, cfg.ChainID)
	if err != nil {
		return fmt.Errorf("failed to generate l1Contracts: %w", err)
	}

	if err := jsonutil.WriteJSON(l1Contracts, ioutil.ToStdOutOrFileOrNoop(cfg.Outfile, 0o666)); err != nil {
		return fmt.Errorf("failed to write L1 contract addresses: %w", err)
	}

	return nil
}

func L1(globalState *state.State, chainID common.Hash) (*L1Contracts, error) {
	chainState, err := globalState.Chain(chainID)
	if err != nil {
		return nil, fmt.Errorf("failed to get chain state for ID %s: %w", chainID.String(), err)
	}

	l1Contracts := L1Contracts{
		SuperchainDeployment: SuperchainDeployment{
			ProxyAdminAddress:            globalState.SuperchainDeployment.ProxyAdminAddress,
			SuperchainConfigProxyAddress: globalState.SuperchainDeployment.SuperchainConfigProxyAddress,
			SuperchainConfigImplAddress:  globalState.SuperchainDeployment.SuperchainConfigImplAddress,
			ProtocolVersionsProxyAddress: globalState.SuperchainDeployment.ProtocolVersionsProxyAddress,
			ProtocolVersionsImplAddress:  globalState.SuperchainDeployment.ProtocolVersionsImplAddress,
		},
		OpChainDeployment: OpChainDeployment{
			ProxyAdminAddress:                        chainState.ProxyAdminAddress,
			AddressManagerAddress:                    chainState.AddressManagerAddress,
			L1ERC721BridgeProxyAddress:               chainState.L1ERC721BridgeProxyAddress,
			SystemConfigProxyAddress:                 chainState.SystemConfigProxyAddress,
			OptimismMintableERC20FactoryProxyAddress: chainState.OptimismMintableERC20FactoryProxyAddress,
			L1StandardBridgeProxyAddress:             chainState.L1StandardBridgeProxyAddress,
			L1CrossDomainMessengerProxyAddress:       chainState.L1CrossDomainMessengerProxyAddress,
			OptimismPortalProxyAddress:               chainState.OptimismPortalProxyAddress,
			DisputeGameFactoryProxyAddress:           chainState.DisputeGameFactoryProxyAddress,
			AnchorStateRegistryProxyAddress:          chainState.AnchorStateRegistryProxyAddress,
			AnchorStateRegistryImplAddress:           chainState.AnchorStateRegistryImplAddress,
			FaultDisputeGameAddress:                  chainState.FaultDisputeGameAddress,
			PermissionedDisputeGameAddress:           chainState.PermissionedDisputeGameAddress,
			DelayedWETHPermissionedGameProxyAddress:  chainState.DelayedWETHPermissionedGameProxyAddress,
			// DelayedWETHPermissionlessGameProxyAddress: chainState.DelayedWETHPermissionlessGameProxyAddress,
		},
		ImplementationsDeployment: ImplementationsDeployment{
			OpcmAddress:                             globalState.ImplementationsDeployment.OpcmAddress,
			DelayedWETHImplAddress:                  globalState.ImplementationsDeployment.DelayedWETHImplAddress,
			OptimismPortalImplAddress:               globalState.ImplementationsDeployment.OptimismPortalImplAddress,
			PreimageOracleSingletonAddress:          globalState.ImplementationsDeployment.PreimageOracleSingletonAddress,
			MipsSingletonAddress:                    globalState.ImplementationsDeployment.MipsSingletonAddress,
			SystemConfigImplAddress:                 globalState.ImplementationsDeployment.SystemConfigImplAddress,
			L1CrossDomainMessengerImplAddress:       globalState.ImplementationsDeployment.L1CrossDomainMessengerImplAddress,
			L1ERC721BridgeImplAddress:               globalState.ImplementationsDeployment.L1ERC721BridgeImplAddress,
			L1StandardBridgeImplAddress:             globalState.ImplementationsDeployment.L1StandardBridgeImplAddress,
			OptimismMintableERC20FactoryImplAddress: globalState.ImplementationsDeployment.OptimismMintableERC20FactoryImplAddress,
			DisputeGameFactoryImplAddress:           globalState.ImplementationsDeployment.DisputeGameFactoryImplAddress,
		},
	}

	return &l1Contracts, nil
}
