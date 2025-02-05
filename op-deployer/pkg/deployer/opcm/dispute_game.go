package opcm

import (
	"github.com/ethereum/go-ethereum/common"

	"github.com/ethereum-optimism/optimism/op-chain-ops/script"
)

type DeployDisputeGameInput struct {
	Release                  string
	StandardVersionsToml     string
	VmAddress                common.Address
	GameKind                 string
	GameType                 uint32
	AbsolutePrestate         common.Hash
	MaxGameDepth             uint64
	SplitDepth               uint64
	ClockExtension           uint64
	MaxClockDuration         uint64
	DelayedWethProxy         common.Address
	AnchorStateRegistryProxy common.Address
	L2ChainId                common.Hash
	Proposer                 common.Address
	Challenger               common.Address
}

func (input *DeployDisputeGameInput) InputSet() bool {
	return true
}

type DeployDisputeGameOutput struct {
	DisputeGameImpl common.Address
}

func (output *DeployDisputeGameOutput) CheckOutput(input common.Address) error {
	return nil
}

func DeployDisputeGame(
	host *script.Host,
	input DeployDisputeGameInput,
) (DeployDisputeGameOutput, error) {
	return RunScriptSingle[DeployDisputeGameInput, DeployDisputeGameOutput](host, input, "DeployDisputeGame.s.sol", "DeployDisputeGame")
}
