package validations

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
)

const (
	VersionV180 = "v1.8.0"
	VersionV200 = "v2.0.0"
)

var addresses = map[uint64]map[string]common.Address{
	11155111: {
		// Both versions below bootstrapped on 02/23/2025 using OP Deployer.
		VersionV180: common.HexToAddress("0x0a5bf8ebb4b177b2dcc6eba933db726a2e2e2b4d"),
		VersionV200: common.HexToAddress("0xaf72eedb110f114a3b4e921c12755b4e47dbd63d"),
	},
}

func ValidatorAddress(chainID uint64, version string) (common.Address, error) {
	chainAddresses, ok := addresses[chainID]
	if !ok {
		return common.Address{}, fmt.Errorf("unsupported chain ID: %d", chainID)
	}

	address, ok := chainAddresses[version]
	if !ok {
		return common.Address{}, fmt.Errorf("unsupported version: %s", version)
	}
	return address, nil
}
