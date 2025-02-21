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
		VersionV180: common.HexToAddress("0x2A788Bb1D32AD0dcEC1A51B7156015Aa90548d8C"),
		VersionV200: common.HexToAddress("0x34FFEEF9D42E0EF0d999fBF01E006f745083Fd9b"),
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
