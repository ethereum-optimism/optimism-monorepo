package boot

import (
	"encoding/binary"
	"encoding/json"
	"math"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-program/chainconfig"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/params"
)

// CustomChainIDIndicator is used to detect when the program should load custom chain configuration
var CustomChainIDIndicator = eth.ChainIDFromUInt64(uint64(math.MaxUint64))

type BootInfo struct {
	L1Head             common.Hash
	L2OutputRoot       common.Hash
	L2Claim            common.Hash
	L2ClaimBlockNumber uint64
	L2ChainID          eth.ChainID

	L2ChainConfig *params.ChainConfig
	RollupConfig  *rollup.Config
}

type BootstrapClient struct {
	r oracleClient
}

func NewBootstrapClient(r oracleClient) *BootstrapClient {
	return &BootstrapClient{r: r}
}

func (br *BootstrapClient) BootInfo() *BootInfo {
	l1Head := common.BytesToHash(br.r.Get(L1HeadLocalIndex))
	l2OutputRoot := common.BytesToHash(br.r.Get(L2OutputRootLocalIndex))
	l2Claim := common.BytesToHash(br.r.Get(L2ClaimLocalIndex))
	l2ClaimBlockNumber := binary.BigEndian.Uint64(br.r.Get(L2ClaimBlockNumberLocalIndex))
	l2ChainID := eth.ChainIDFromUInt64(binary.BigEndian.Uint64(br.r.Get(L2ChainIDLocalIndex)))

	var l2ChainConfig *params.ChainConfig
	var rollupConfig *rollup.Config
	if l2ChainID == CustomChainIDIndicator {
		l2ChainConfig = new(params.ChainConfig)
		err := json.Unmarshal(br.r.Get(L2ChainConfigLocalIndex), &l2ChainConfig)
		if err != nil {
			panic("failed to bootstrap l2ChainConfig")
		}
		rollupConfig = new(rollup.Config)
		err = json.Unmarshal(br.r.Get(RollupConfigLocalIndex), rollupConfig)
		if err != nil {
			panic("failed to bootstrap rollup config")
		}
	} else {
		var err error
		rollupConfig, err = chainconfig.RollupConfigByChainID(l2ChainID)
		if err != nil {
			panic(err)
		}
		l2ChainConfig, err = chainconfig.ChainConfigByChainID(l2ChainID)
		if err != nil {
			panic(err)
		}
	}

	return &BootInfo{
		L1Head:             l1Head,
		L2OutputRoot:       l2OutputRoot,
		L2Claim:            l2Claim,
		L2ClaimBlockNumber: l2ClaimBlockNumber,
		L2ChainID:          l2ChainID,
		L2ChainConfig:      l2ChainConfig,
		RollupConfig:       rollupConfig,
	}
}
