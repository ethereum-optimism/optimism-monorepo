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
		"v1.8.0": common.HexToAddress("0xe6c2eb5eef0c51fbdb27bbc27f24a0ad70fe6f38"),
		"v2.0.0": common.HexToAddress("0xb142194236930c0a3e83b2635778434Eb146a1FE"),
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
