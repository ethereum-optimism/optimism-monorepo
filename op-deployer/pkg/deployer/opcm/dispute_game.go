package opcm

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"

	"github.com/ethereum-optimism/optimism/op-chain-ops/script"
)

type DeployDisputeGameInput struct {
	Release                  string
	StandardVersionsToml     string
	MipsVersion              uint64
	MinProposalSizeBytes     uint64
	ChallengePeriodSeconds   uint64
	GameKind                 string
	GameType                 uint32
	AbsolutePrestate         common.Hash
	MaxGameDepth             uint64
	SplitDepth               uint64
	ClockExtension           uint64
	MaxClockDuration         uint64
	DelayedWethProxy         common.Address
	AnchorStateRegistryProxy common.Address
	L2ChainId                uint64
	Proposer                 common.Address
	Challenger               common.Address
}

func (input *DeployDisputeGameInput) InputSet() bool {
	return true
}

type DeployDisputeGameOutput struct {
	DisputeGameImpl         common.Address
	MipsSingleton           common.Address
	PreimageOracleSingleton common.Address
}

func (output *DeployDisputeGameOutput) CheckOutput(input common.Address) error {
	return nil
}

type DeployDisputeGameScript struct {
	Run func(input, output common.Address) error
}

func DeployDisputeGame(
	host *script.Host,
	input DeployDisputeGameInput,
) (DeployDisputeGameOutput, error) {
	var output DeployDisputeGameOutput
	inputAddr := host.NewScriptAddress()
	outputAddr := host.NewScriptAddress()

	cleanupInput, err := script.WithPrecompileAtAddress[*DeployDisputeGameInput](host, inputAddr, &input)
	if err != nil {
		return output, fmt.Errorf("failed to insert DeployDisputeGameInput precompile: %w", err)
	}
	defer cleanupInput()

	cleanupOutput, err := script.WithPrecompileAtAddress[*DeployDisputeGameOutput](host, outputAddr, &output,
		script.WithFieldSetter[*DeployDisputeGameOutput])
	if err != nil {
		return output, fmt.Errorf("failed to insert DeployDisputeGameOutput precompile: %w", err)
	}
	defer cleanupOutput()

	implContract := "DeployDisputeGame"
	deployScript, cleanupDeploy, err := script.WithScript[DeployDisputeGameScript](host, "DeployDisputeGame.s.sol", implContract)
	if err != nil {
		return output, fmt.Errorf("failed to load %s script: %w", implContract, err)
	}
	defer cleanupDeploy()

	if err := deployScript.Run(inputAddr, outputAddr); err != nil {
		return output, fmt.Errorf("failed to run %s script: %w", implContract, err)
	}

	return output, nil
}
