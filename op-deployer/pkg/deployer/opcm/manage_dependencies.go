package opcm

import (
	"math/big"

	"github.com/ethereum-optimism/optimism/op-chain-ops/script"
	"github.com/ethereum/go-ethereum/common"
)

type ManageDependenciesInput struct {
	ChainId      *big.Int
	SystemConfig common.Address
	Remove       bool
}

func ManageDependencies(
	host *script.Host,
	input ManageDependenciesInput,
) error {
	return RunScriptVoid[ManageDependenciesInput](host, input, "ManageDependencies.s.sol", "ManageDependencies")
}
