package tests

import (
	"context"
	"fmt"
	"math/big"

	nat "github.com/ethereum-optimism/optimism/op-nat"
	"github.com/ethereum-optimism/optimism/op-nat/network"
	"github.com/ethereum/go-ethereum/log"
)

// NetworkGetChainID is a test that checks if the RPC call `ChainID` is working.
var NetworkGetChainID = nat.Test{
	ID: "network-get-chainid",
	Fn: func(ctx context.Context, log log.Logger, config nat.Config) (bool, error) {
		network, err := network.NewNetwork(ctx, log, config.RPCURL, "kurtosis-l2")
		if err != nil {
			return false, fmt.Errorf("failed to setup network")
		}
		chainID, err := network.RPC.ChainID(ctx)
		expectedChainID, ok := new(big.Int).SetString(config.SC.L2[0].ID, 10)
		if !ok {
			return false, fmt.Errorf("failed to parse expected chain id")
		}
		if err != nil || chainID == nil || chainID.Cmp(expectedChainID) != 0 {
			return false, fmt.Errorf("failed to get chain id")
		}
		log.Debug("successfully got the chain id", "chain_id", chainID.String())
		return true, nil
	},
}
