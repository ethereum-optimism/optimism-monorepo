package opcm

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"

	"github.com/ethereum-optimism/optimism/op-chain-ops/script"
)

type DeployProxyInput struct {
	Owner common.Address
}

func (input *DeployProxyInput) InputSet() bool {
	return true
}

type DeployProxyOutput struct {
	Proxy common.Address
}

func (output *DeployProxyOutput) CheckOutput(input common.Address) error {
	if output.Proxy == (common.Address{}) {
		return fmt.Errorf("output.Proxy not set")
	}
	return nil
}

type DeployProxyScript struct {
	Run func(input, output common.Address) error
}

func DeployProxy(
	host *script.Host,
	input DeployProxyInput,
) (DeployProxyOutput, error) {
	var output DeployProxyOutput
	inputAddr := host.NewScriptAddress()
	outputAddr := host.NewScriptAddress()

	cleanupInput, err := script.WithPrecompileAtAddress[*DeployProxyInput](host, inputAddr, &input)
	if err != nil {
		return output, fmt.Errorf("failed to insert DeployProxyInput precompile: %w", err)
	}
	defer cleanupInput()

	cleanupOutput, err := script.WithPrecompileAtAddress[*DeployProxyOutput](host, outputAddr, &output,
		script.WithFieldSetter[*DeployProxyOutput])
	if err != nil {
		return output, fmt.Errorf("failed to insert DeployProxyOutput precompile: %w", err)
	}
	defer cleanupOutput()

	implContract := "DeployProxy"
	deployScript, cleanupDeploy, err := script.WithScript[DeployProxyScript](host, "DeployProxy.s.sol", implContract)
	if err != nil {
		return output, fmt.Errorf("failed to load %s script: %w", implContract, err)
	}
	defer cleanupDeploy()

	if err := deployScript.Run(inputAddr, outputAddr); err != nil {
		return output, fmt.Errorf("failed to run %s script: %w", implContract, err)
	}

	return output, nil
}
