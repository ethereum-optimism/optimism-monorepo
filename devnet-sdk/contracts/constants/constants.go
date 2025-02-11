package constants

import (
	"github.com/ethereum-optimism/optimism/devnet-sdk/types"
	"github.com/ethereum/go-ethereum/common"
)

var (
	L2ToL2CrossDomainMessenger types.Address = common.HexToAddress("0x4200000000000000000000000000000000000023")
	SuperchainWETH             types.Address = common.HexToAddress("0x4200000000000000000000000000000000000024")
	ETHLiquidity               types.Address = common.HexToAddress("0x4200000000000000000000000000000000000025")
	SuperchainTokenBridge      types.Address = common.HexToAddress("0x4200000000000000000000000000000000000028")
)

const (
	ETH  = 1e18
	Gwei = 1e9
)
