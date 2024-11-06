package opcm

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"

	"github.com/ethereum-optimism/optimism/op-chain-ops/script"
)

type DeployMIPSInput struct {
	Release                string
	StandardVersionsToml   string
	MipsVersion            uint8
	MinProposalSizeBytes   uint64
	ChallengePeriodSeconds uint64
}

func (input *DeployMIPSInput) InputSet() bool {
	return true
}

type DeployMIPSOutput struct {
	MipsSingleton           common.Address
	PreimageOracleSingleton common.Address
}

func (output *DeployMIPSOutput) CheckOutput(input common.Address) error {
	return nil
}

type DeployMIPSScript struct {
	Run func(input, output common.Address) error
}

func DeployMIPS(
	host *script.Host,
	input DeployMIPSInput,
) (DeployMIPSOutput, error) {
	var output DeployMIPSOutput
	inputAddr := host.NewScriptAddress()
	outputAddr := host.NewScriptAddress()

	cleanupInput, err := script.WithPrecompileAtAddress[*DeployMIPSInput](host, inputAddr, &input)
	if err != nil {
		return output, fmt.Errorf("failed to insert DeployMIPSInput precompile: %w", err)
	}
	defer cleanupInput()

	cleanupOutput, err := script.WithPrecompileAtAddress[*DeployMIPSOutput](host, outputAddr, &output,
		script.WithFieldSetter[*DeployMIPSOutput])
	if err != nil {
		return output, fmt.Errorf("failed to insert DeployMIPSOutput precompile: %w", err)
	}
	defer cleanupOutput()

	implContract := "DeployMIPS"
	deployScript, cleanupDeploy, err := script.WithScript[DeployMIPSScript](host, "DeployMIPS.s.sol", implContract)
	if err != nil {
		return output, fmt.Errorf("failed to load %s script: %w", implContract, err)
	}
	defer cleanupDeploy()

	if err := deployScript.Run(inputAddr, outputAddr); err != nil {
		return output, fmt.Errorf("failed to run %s script: %w", implContract, err)
	}

	return output, nil
}
