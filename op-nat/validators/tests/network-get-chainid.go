package tests

import (
	"context"
	"fmt"

	nat "github.com/ethereum-optimism/optimism/op-nat"
	"github.com/ethereum-optimism/optimism/op-nat/network"
	"github.com/ethereum/go-ethereum/log"
)

// NetworkGetChainID is a test that checks if the RPC call `ChainID` is working.
var NetworkGetChainID = nat.Test{
	ID: "network-get-chainid",
	Fn: func(ctx context.Context, log log.Logger, config nat.Config, _ interface{}) (bool, error) {

		for _, network := range config.GetNetworks() {
			log.Info("validating network",
				"addr", network.Addr,
				"chain_id", network.ChainID,
			)
			if ok, err := ValidateChainID(ctx, network); !ok || err != nil {
				return false, err
			}

		}
		return true, nil

	},
}

func ValidateChainID(ctx context.Context, network *network.Network) (bool, error) {
	chainID, err := network.RPC.ChainID(ctx)
	if err != nil {
		return false, fmt.Errorf("error requesting chain id from network %s. Error: %w",
			network.Name,
			err,
		)
	}
	if chainID == nil || (*network.ChainID).Cmp(chainID) != 0 {
		return false, fmt.Errorf("failed to get expected chain id for network %s. Expected: %d. Got: %d",
			network.Name,
			*network.ChainID,
			chainID,
		)
	}
	log.Debug("successfully got the chain id", "chain_id", chainID.String())

	return true, nil
}
