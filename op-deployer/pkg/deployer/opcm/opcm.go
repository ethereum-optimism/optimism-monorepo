package opcm

import (
	"fmt"

	"github.com/ethereum-optimism/optimism/op-chain-ops/script"
	"github.com/ethereum/go-ethereum/common"
)

type DeployOPCMInput struct {
	SuperchainConfig   common.Address
	ProtocolVersions   common.Address
	L1ContractsRelease string

	AddressManagerBlueprint           common.Address
	ProxyBlueprint                    common.Address
	ProxyAdminBlueprint               common.Address
	L1ChugSplashProxyBlueprint        common.Address
	ResolvedDelegateProxyBlueprint    common.Address
	AnchorStateRegistryBlueprint      common.Address
	PermissionedDisputeGame1Blueprint common.Address
	PermissionedDisputeGame2Blueprint common.Address

	L1ERC721BridgeImpl               common.Address
	OptimismPortalImpl               common.Address
	SystemConfigImpl                 common.Address
	OptimismMintableERC20FactoryImpl common.Address
	L1CrossDomainMessengerImpl       common.Address
	L1StandardBridgeImpl             common.Address
	DisputeGameFactoryImpl           common.Address
	DelayedWETHImpl                  common.Address
	MipsImpl                         common.Address
}

type DeployOPCMOutput struct {
	Opcm common.Address
}

func DeployOPCM(
	host *script.Host,
	input DeployOPCMInput,
) (DeployOPCMOutput, error) {
	scriptFile := "DeployOPCM.s.sol"
	contractName := "DeployOPCM"

	out, err := RunBasicScript[DeployOPCMInput, DeployOPCMOutput](host, input, scriptFile, contractName)
	if err != nil {
		return DeployOPCMOutput{}, fmt.Errorf("failed to deploy OPCM: %w", err)
	}

	if err := host.RememberOnLabel("OPContractsManager", "OPContractsManager.sol", "OPContractsManager"); err != nil {
		return DeployOPCMOutput{}, fmt.Errorf("failed to link OPContractsManager label: %w", err)
	}

	return out, nil
}
