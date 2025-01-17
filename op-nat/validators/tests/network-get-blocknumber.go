package tests

import (
	"context"
	"fmt"

	nat "github.com/ethereum-optimism/optimism/op-nat"
	"github.com/ethereum/go-ethereum/log"
)

// NetworkGetBlockNumber is a test that checks if RPC call `BlockNumber` is working.
var NetworkGetBlockNumber = nat.Test{
	ID: "network-get-block-number",
	Fn: func(ctx context.Context, log log.Logger, config nat.Config, _ interface{}) (bool, error) {

		for _, network := range config.GetNetworks() {
			blockID, err := network.RPC.BlockNumber(ctx)
			if err != nil {
				return false, fmt.Errorf("failed to get block number")
			}
			log.Debug("successfully got the block number", "blockNumber", blockID)
		}
		return true, nil
	},
}
