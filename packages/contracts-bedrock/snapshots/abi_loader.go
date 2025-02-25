package snapshots

import (
	"bytes"
	_ "embed"

	"github.com/ethereum/go-ethereum/accounts/abi"
)

//go:embed abi/DisputeGameFactory.json
var disputeGameFactory []byte

//go:embed abi/FaultDisputeGame.json
var faultDisputeGame []byte

//go:embed abi/SuperFaultDisputeGame.json
var superFaultDisputeGame []byte

//go:embed abi/PreimageOracle.json
var preimageOracle []byte

//go:embed abi/MIPS.json
var mips []byte

//go:embed abi/DelayedWETH.json
var delayedWETH []byte

//go:embed abi/SystemConfig.json
var systemConfig []byte

//go:embed abi/CrossL2Inbox.json
var crossL2Inbox []byte

func LoadDisputeGameFactoryABI() *abi.ABI {
	return loadABI(disputeGameFactory)
}
func LoadFaultDisputeGameABI() *abi.ABI {
	return loadABI(faultDisputeGame)
}
func LoadSuperFaultDisputeGame() *abi.ABI {
	return loadABI(superFaultDisputeGame)
}
func LoadPreimageOracleABI() *abi.ABI {
	return loadABI(preimageOracle)
}
func LoadMIPSABI() *abi.ABI {
	return loadABI(mips)
}
func LoadDelayedWETHABI() *abi.ABI {
	return loadABI(delayedWETH)
}

func LoadSystemConfigABI() *abi.ABI {
	return loadABI(systemConfig)
}

func LoadCrossL2InboxABI() *abi.ABI {
	return loadABI(crossL2Inbox)
}

func loadABI(json []byte) *abi.ABI {
	if parsed, err := abi.JSON(bytes.NewReader(json)); err != nil {
		panic(err)
	} else {
		return &parsed
	}
}
