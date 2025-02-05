package opcm

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	"github.com/ethereum-optimism/optimism/op-chain-ops/script"
)

type DeployDelayedWETHInput struct {
	Release               string
	ProxyAdmin            common.Address
	SuperchainConfigProxy common.Address
	DelayedWethImpl       common.Address
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

func DeployDelayedWETH(
	host *script.Host,
	input DeployDelayedWETHInput,
) (DeployDelayedWETHOutput, error) {
	return RunScriptSingle[DeployDelayedWETHInput, DeployDelayedWETHOutput](host, input, "DeployDelayedWETH.s.sol", "DeployDelayedWETH")
}
