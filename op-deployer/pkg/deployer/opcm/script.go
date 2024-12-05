package opcm

import (
	"fmt"

	"github.com/ethereum-optimism/optimism/op-chain-ops/script"
	"github.com/ethereum/go-ethereum/common"
)

type BasicScriptIO struct {
	Run func(input, output common.Address) error
}

func RunBasicScript[I any, O any](
	host *script.Host,
	input I,
	scriptFile string,
	contractName string,
) (O, error) {
	var output O
	inputAddr := host.NewScriptAddress()
	outputAddr := host.NewScriptAddress()

	cleanupInput, err := script.WithPrecompileAtAddress[*I](host, inputAddr, &input)
	if err != nil {
		return output, fmt.Errorf("failed to insert input precompile: %w", err)
	}
	defer cleanupInput()

	cleanupOutput, err := script.WithPrecompileAtAddress[*O](host, outputAddr, &output,
		script.WithFieldSetter[*O])
	if err != nil {
		return output, fmt.Errorf("failed to insert output precompile: %w", err)
	}
	defer cleanupOutput()

	deployScript, cleanupDeploy, err := script.WithScript[BasicScriptIO](host, scriptFile, contractName)
	if err != nil {
		return output, fmt.Errorf("failed to load %s script: %w", scriptFile, err)
	}
	defer cleanupDeploy()

	if err := deployScript.Run(inputAddr, outputAddr); err != nil {
		return output, fmt.Errorf("failed to run %s script: %w", scriptFile, err)
	}

	return output, nil
}
