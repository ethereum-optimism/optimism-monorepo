package opcm

import (
	"github.com/ethereum/go-ethereum/common"

	"github.com/ethereum-optimism/optimism/op-chain-ops/script"
)

type DeployMIPSInput struct {
	MipsVersion    uint64
	PreimageOracle common.Address
}

type DeployMIPSOutput struct {
	MipsSingleton common.Address
}

func DeployMIPS(
	host *script.Host,
	input DeployMIPSInput,
) (DeployMIPSOutput, error) {
	return RunScriptSingle[DeployMIPSInput, DeployMIPSOutput](host, input, "DeployMIPS.s.sol", "DeployMIPS")
}
