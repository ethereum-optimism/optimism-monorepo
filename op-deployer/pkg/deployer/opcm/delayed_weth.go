package opcm

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	"github.com/ethereum-optimism/optimism/op-chain-ops/script"
)

type DeployDelayedWETHInput struct {
	Release               string
	StandardVersionsToml  string
	ProxyAdmin            common.Address
	SuperchainConfigProxy common.Address
	DelayedWethOwner      common.Address
	DelayedWethDelay      *big.Int
}

func (input *DeployDelayedWETHInput) InputSet() bool {
	return true
}

type DeployDelayedWETHOutput struct {
	DelayedWethImpl  common.Address
	DelayedWethProxy common.Address
}

func (output *DeployDelayedWETHOutput) CheckOutput(input common.Address) error {
	return nil
}

type DeployDelayedWETHScript struct {
	Run func(input, output common.Address) error
}

func DeployDelayedWETH(
	host *script.Host,
	input DeployDelayedWETHInput,
) (DeployDelayedWETHOutput, error) {
	var output DeployDelayedWETHOutput
	inputAddr := host.NewScriptAddress()
	outputAddr := host.NewScriptAddress()

	cleanupInput, err := script.WithPrecompileAtAddress[*DeployDelayedWETHInput](host, inputAddr, &input)
	if err != nil {
		return output, fmt.Errorf("failed to insert DeployDelayedWETHInput precompile: %w", err)
	}
	defer cleanupInput()

	cleanupOutput, err := script.WithPrecompileAtAddress[*DeployDelayedWETHOutput](host, outputAddr, &output,
		script.WithFieldSetter[*DeployDelayedWETHOutput])
	if err != nil {
		return output, fmt.Errorf("failed to insert DeployDelayedWETHOutput precompile: %w", err)
	}
	defer cleanupOutput()

	implContract := "DeployDelayedWETH"
	deployScript, cleanupDeploy, err := script.WithScript[DeployDelayedWETHScript](host, "DeployDelayedWETH.s.sol", implContract)
	if err != nil {
		return output, fmt.Errorf("failed to load %s script: %w", implContract, err)
	}
	defer cleanupDeploy()

	if err := deployScript.Run(inputAddr, outputAddr); err != nil {
		return output, fmt.Errorf("failed to run %s script: %w", implContract, err)
	}

	return output, nil
}
