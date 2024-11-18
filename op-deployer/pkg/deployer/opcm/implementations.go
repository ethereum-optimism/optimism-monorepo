package opcm

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	"github.com/ethereum-optimism/optimism/op-chain-ops/script"
)

type DeployImplementationsInput struct {
	Salt                            common.Hash
	WithdrawalDelaySeconds          *big.Int
	MinProposalSizeBytes            *big.Int
	ChallengePeriodSeconds          *big.Int
	ProofMaturityDelaySeconds       *big.Int
	DisputeGameFinalityDelaySeconds *big.Int
	MipsVersion                     *big.Int
	// Release version to set OPCM implementations for, of the format `op-contracts/vX.Y.Z`.
	L1ContractsRelease    string
	SuperchainConfigProxy common.Address
	ProtocolVersionsProxy common.Address
	UseInterop            bool // if true, deploy Interop implementations

	StandardVersionsToml string // contents of 'standard-versions-mainnet.toml' or 'standard-versions-sepolia.toml' file
}

func (input *DeployImplementationsInput) InputSet() bool {
	return true
}

type DeployImplementationsOutput struct {
	Opcm                             common.Address
	DelayedWETHImpl                  common.Address
	OptimismPortalImpl               common.Address
	PreimageOracleSingleton          common.Address
	MipsSingleton                    common.Address
	SystemConfigImpl                 common.Address
	L1CrossDomainMessengerImpl       common.Address
	L1ERC721BridgeImpl               common.Address
	L1StandardBridgeImpl             common.Address
	OptimismMintableERC20FactoryImpl common.Address
	DisputeGameFactoryImpl           common.Address
}

func (output *DeployImplementationsOutput) CheckOutput(input common.Address) error {
	return nil
}

type DeployImplementationsScript struct {
	Run func(input, output common.Address) error
}

func DeployImplementations(
	host *script.Host,
	input DeployImplementationsInput,
) (DeployImplementationsOutput, error) {
	var output DeployImplementationsOutput
	inputAddr := host.NewScriptAddress()
	outputAddr := host.NewScriptAddress()

	cleanupInput, err := script.WithPrecompileAtAddress[*DeployImplementationsInput](host, inputAddr, &input)
	if err != nil {
		return output, fmt.Errorf("failed to insert DeployImplementationsInput precompile: %w", err)
	}
	defer cleanupInput()

	cleanupOutput, err := script.WithPrecompileAtAddress[*DeployImplementationsOutput](host, outputAddr, &output,
		script.WithFieldSetter[*DeployImplementationsOutput])
	if err != nil {
		return output, fmt.Errorf("failed to insert DeployImplementationsOutput precompile: %w", err)
	}
	defer cleanupOutput()

	implContract := "DeployImplementations"
	if input.UseInterop {
		implContract = "DeployImplementationsInterop"
	}
	deployScript, cleanupDeploy, err := script.WithScript[DeployImplementationsScript](host, "DeployImplementations.s.sol", implContract)
	if err != nil {
		return output, fmt.Errorf("failed to load %s script: %w", implContract, err)
	}
	defer cleanupDeploy()

	opcmContract := "OPContractsManager"
	if input.UseInterop {
		opcmContract = "OPContractsManagerInterop"
	}
	if err := host.RememberOnLabel("OPContractsManager", opcmContract+".sol", opcmContract); err != nil {
		return output, fmt.Errorf("failed to link OPContractsManager label: %w", err)
	}

	// So we can see in detail where the SystemConfig interop initializer fails
	sysConfig := "SystemConfig"
	if input.UseInterop {
		sysConfig = "SystemConfigInterop"
	}
	if err := host.RememberOnLabel("SystemConfigImpl", sysConfig+".sol", sysConfig); err != nil {
		return output, fmt.Errorf("failed to link SystemConfig label: %w", err)
	}

	if err := deployScript.Run(inputAddr, outputAddr); err != nil {
		return output, fmt.Errorf("failed to run %s script: %w", implContract, err)
	}

	return output, nil
}
