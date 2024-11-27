package opcm

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"

	"github.com/ethereum-optimism/optimism/op-chain-ops/script"
)

type DeployAsteriscInput struct {
	PreimageOracle common.Address
}

func (input *DeployAsteriscInput) InputSet() bool {
	return true
}

type DeployAsteriscOutput struct {
	AsteriscSingleton common.Address
}

func (output *DeployAsteriscOutput) CheckOutput(input common.Address) error {
	return nil
}

type DeployAsteriscScript struct {
	Run func(input, output common.Address) error
}

func DeployAsterisc(
	host *script.Host,
	input DeployAsteriscInput,
) (DeployAsteriscOutput, error) {
	var output DeployAsteriscOutput
	inputAddr := host.NewScriptAddress()
	outputAddr := host.NewScriptAddress()

	cleanupInput, err := script.WithPrecompileAtAddress[*DeployAsteriscInput](host, inputAddr, &input)
	if err != nil {
		return output, fmt.Errorf("failed to insert DeployAsteriscInput precompile: %w", err)
	}
	defer cleanupInput()

	cleanupOutput, err := script.WithPrecompileAtAddress[*DeployAsteriscOutput](host, outputAddr, &output,
		script.WithFieldSetter[*DeployAsteriscOutput])
	if err != nil {
		return output, fmt.Errorf("failed to insert DeployAsteriscOutput precompile: %w", err)
	}
	defer cleanupOutput()

	implContract := "DeployAsterisc"
	deployScript, cleanupDeploy, err := script.WithScript[DeployAsteriscScript](host, "DeployAsterisc.s.sol", implContract)
	if err != nil {
		return output, fmt.Errorf("failed to load %s script: %w", implContract, err)
	}
	defer cleanupDeploy()

	if err := deployScript.Run(inputAddr, outputAddr); err != nil {
		return output, fmt.Errorf("failed to run %s script: %w", implContract, err)
	}

	return output, nil
}
