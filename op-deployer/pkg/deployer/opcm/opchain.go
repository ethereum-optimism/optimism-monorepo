package opcm

import (
	_ "embed"
	"fmt"
	"math/big"

	"github.com/ethereum-optimism/optimism/op-chain-ops/script"
	"github.com/ethereum/go-ethereum/common"
)

// PermissionedGameStartingAnchorRoot is a root of bytes32(hex"dead") for the permissioned game at block 0,
// and no root for the permissionless game.
var PermissionedGameStartingAnchorRoot = []byte{
	0xde, 0xad, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
}

type DeployOPChainInput struct {
	OpChainProxyAdminOwner common.Address
	SystemConfigOwner      common.Address
	Batcher                common.Address
	UnsafeBlockSigner      common.Address
	Proposer               common.Address
	Challenger             common.Address

	BasefeeScalar     uint32
	BlobBaseFeeScalar uint32
	L2ChainId         *big.Int
	Opcm              common.Address
	SaltMixer         string
	GasLimit          uint64

	DisputeGameType              uint32
	DisputeAbsolutePrestate      common.Hash
	DisputeMaxGameDepth          uint64
	DisputeSplitDepth            uint64
	DisputeClockExtension        uint64
	DisputeMaxClockDuration      uint64
	AllowCustomDisputeParameters bool
}

func (input *DeployOPChainInput) InputSet() bool {
	return true
}

func (input *DeployOPChainInput) StartingAnchorRoot() []byte {
	return PermissionedGameStartingAnchorRoot
}

type DeployOPChainOutput struct {
	OpChainProxyAdmin                 common.Address
	AddressManager                    common.Address
	L1ERC721BridgeProxy               common.Address
	SystemConfigProxy                 common.Address
	OptimismMintableERC20FactoryProxy common.Address
	L1StandardBridgeProxy             common.Address
	L1CrossDomainMessengerProxy       common.Address
	// Fault proof contracts below.
	OptimismPortalProxy                common.Address
	DisputeGameFactoryProxy            common.Address
	AnchorStateRegistryProxy           common.Address
	FaultDisputeGame                   common.Address
	PermissionedDisputeGame            common.Address
	DelayedWETHPermissionedGameProxy   common.Address
	DelayedWETHPermissionlessGameProxy common.Address
}

func (output *DeployOPChainOutput) CheckOutput(input common.Address) error {
	return nil
}

type DeployOPChainScript struct {
	Run func(input, output common.Address) error
}

func DeployOPChain(host *script.Host, input DeployOPChainInput) (DeployOPChainOutput, error) {
	return RunScriptSingle[DeployOPChainInput, DeployOPChainOutput](host, input, "DeployOPChain.s.sol", "DeployOPChain")
}

type ReadImplementationAddressesInput struct {
	DeployOPChainOutput
	Opcm    common.Address
	Release string
}

type ReadImplementationAddressesOutput struct {
	DelayedWETH                  common.Address
	OptimismPortal               common.Address
	SystemConfig                 common.Address
	L1CrossDomainMessenger       common.Address
	L1ERC721Bridge               common.Address
	L1StandardBridge             common.Address
	OptimismMintableERC20Factory common.Address
	DisputeGameFactory           common.Address
	MipsSingleton                common.Address
	PreimageOracleSingleton      common.Address
}

type ReadImplementationAddressesScript struct {
	Run func(input, output common.Address) error
}

func ReadImplementationAddresses(host *script.Host, input ReadImplementationAddressesInput) (ReadImplementationAddressesOutput, error) {
	var rio ReadImplementationAddressesOutput
	inputAddr := host.NewScriptAddress()
	outputAddr := host.NewScriptAddress()

	cleanupInput, err := script.WithPrecompileAtAddress[*ReadImplementationAddressesInput](host, inputAddr, &input)
	if err != nil {
		return rio, fmt.Errorf("failed to insert ReadImplementationAddressesInput precompile: %w", err)
	}
	defer cleanupInput()
	host.Label(inputAddr, "ReadImplementationAddressesInput")

	cleanupOutput, err := script.WithPrecompileAtAddress[*ReadImplementationAddressesOutput](host, outputAddr, &rio,
		script.WithFieldSetter[*ReadImplementationAddressesOutput])
	if err != nil {
		return rio, fmt.Errorf("failed to insert ReadImplementationAddressesOutput precompile: %w", err)
	}
	defer cleanupOutput()
	host.Label(outputAddr, "ReadImplementationAddressesOutput")

	deployScript, cleanupDeploy, err := script.WithScript[ReadImplementationAddressesScript](host, "ReadImplementationAddresses.s.sol", "ReadImplementationAddresses")
	if err != nil {
		return rio, fmt.Errorf("failed to load ReadImplementationAddresses script: %w", err)
	}
	defer cleanupDeploy()

	if err := deployScript.Run(inputAddr, outputAddr); err != nil {
		return rio, fmt.Errorf("failed to run ReadImplementationAddresses script: %w", err)
	}

	return rio, nil
}
