package opcm

import (
	"fmt"
	"math/big"

	"github.com/ethereum-optimism/optimism/op-chain-ops/script"
	"github.com/ethereum/go-ethereum/common"
)

type DeployAltDAInput struct {
	Salt                     common.Hash
	ProxyAdmin               common.Address
	ChallengeContractOwner   common.Address
	ChallengeWindow          *big.Int
	ResolveWindow            *big.Int
	BondSize                 *big.Int
	ResolverRefundPercentage *big.Int
}

type DeployAltDAOutput struct {
	DataAvailabilityChallengeProxy common.Address
	DataAvailabilityChallengeImpl  common.Address
}

type DeployAltDAScript struct {
	Run func(input, output common.Address) error
}

func DeployAltDA(
	host *script.Host,
	input DeployAltDAInput,
) (DeployAltDAOutput, error) {
	var output DeployAltDAOutput
	inputAddr := host.NewScriptAddress()
	outputAddr := host.NewScriptAddress()

	cleanupInput, err := script.WithPrecompileAtAddress[*DeployAltDAInput](host, inputAddr, &input)
	if err != nil {
		return output, fmt.Errorf("failed to insert DeployAltDAInput precompile: %w", err)
	}
	defer cleanupInput()

	cleanupOutput, err := script.WithPrecompileAtAddress[*DeployAltDAOutput](host, outputAddr, &output,
		script.WithFieldSetter[*DeployAltDAOutput])
	if err != nil {
		return output, fmt.Errorf("failed to insert DeployAltDAOutput precompile: %w", err)
	}
	defer cleanupOutput()

	implContract := "DeployAltDA"
	deployScript, cleanupDeploy, err := script.WithScript[DeployAltDAScript](host, "DeployAltDA.s.sol", implContract)
	if err != nil {
		return output, fmt.Errorf("failed to laod %s script: %w", implContract, err)
	}
	defer cleanupDeploy()

	if err := deployScript.Run(inputAddr, outputAddr); err != nil {
		return output, fmt.Errorf("failed to run %s script: %w", implContract, err)
	}

	return output, nil
}
