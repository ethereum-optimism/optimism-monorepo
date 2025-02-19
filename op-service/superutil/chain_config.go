package superutil

import (
	"fmt"

	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/superchain"
)

func LoadOPStackChainConfigFromChainID(chainID uint64) (*params.ChainConfig, error) {
	chain, err := superchain.GetChain(chainID)
	if err != nil {
		return nil, fmt.Errorf("unable to get chain %d from superchain registry: %w", chainID, err)
	}
	chainCfg, err := chain.Config()
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve chain %d config: %w", chainID, err)
	}
	return params.LoadOPStackChainConfig(chainCfg)
}
