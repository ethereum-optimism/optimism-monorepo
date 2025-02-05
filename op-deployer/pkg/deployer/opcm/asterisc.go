package opcm

import (
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

func DeployAsterisc(
	host *script.Host,
	input DeployAsteriscInput,
) (DeployAsteriscOutput, error) {
	return RunScriptSingle[DeployAsteriscInput, DeployAsteriscOutput](host, input, "DeployAsterisc.s.sol", "DeployAsterisc")
}
