package boot

import preimage "github.com/ethereum-optimism/optimism/op-preimage"

const (
	L1HeadLocalIndex preimage.LocalIndexKey = iota + 1
	L2OutputRootLocalIndex
	L2ClaimLocalIndex
	L2ClaimBlockNumberLocalIndex
	L2ChainIDLocalIndex

	// These local keys are only used for custom chains
	L2ChainConfigLocalIndex
	RollupConfigLocalIndex
	DependencySetLocalIndex
)

type oracleClient interface {
	Get(key preimage.Key) []byte
}
