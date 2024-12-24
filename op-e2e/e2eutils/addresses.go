package e2eutils

import (
	"bytes"
	"sort"

	"golang.org/x/exp/maps"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"

	"github.com/ethereum-optimism/optimism/op-chain-ops/crossdomain"
	"github.com/ethereum-optimism/optimism/op-service/predeploys"
)

func collectAllocAddrs(alloc types.GenesisAlloc) []common.Address {
	var out []common.Address
	out = maps.Keys(alloc)
	// make output deterministic
	sort.Slice(out, func(i, j int) bool {
		return bytes.Compare(out[i][:], out[j][:]) < 0
	})
	return out
}

// CollectAddresses constructs a lists of addresses that may be used as fuzzing corpora
// or random address selection.
func CollectAddresses(sd *SetupData, dp *DeployParams) (out []common.Address) {
	// This should be seeded with:
	//  - reserve 0 for selecting nil (contract creation)
	out = append(out, common.Address{})
	//  - zero address
	out = append(out, common.Address{})
	//  - addresses of signing accounts
	out = append(out, dp.Addresses.All()...)
	// prefunded L1/L2 accounts for testing
	out = append(out, collectAllocAddrs(sd.L1Cfg.Alloc)...)
	out = append(out, collectAllocAddrs(sd.L2Cfg.Alloc)...)

	//  - addresses of system contracts
	out = append(out,
		sd.L1Cfg.Coinbase,
		sd.L2Cfg.Coinbase,
		dp.Addresses.SequencerP2P,
		predeploys.SequencerFeeVaultAddr,
		sd.RollupCfg.BatchInboxAddress,
		sd.RollupCfg.Genesis.SystemConfig.BatcherAddr,
		sd.RollupCfg.DepositContractAddress,
	)
	//  - precompiles
	for i := 0; i <= 0xff; i++ {
		out = append(out, common.Address{19: byte(i)})
	}
	//  - masked L2 version of all the original addrs
	original := out[:]
	for _, addr := range original {
		masked := crossdomain.ApplyL1ToL2Alias(addr)
		out = append(out, masked)
	}
	//  - unmasked L1 version of all the original addrs
	for _, addr := range original {
		unmasked := crossdomain.UndoL1ToL2Alias(addr)
		out = append(out, unmasked)
	}
	return out
}
