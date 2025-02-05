package test

import "github.com/ethereum-optimism/optimism/op-node/rollup"

// ChainSpec wraps a *rollup.ChainSpec, allowing to optionally override individual values,
// otherwise just returning the underlying ChainSpec's values.
type ChainSpec struct {
	*rollup.ChainSpec

	MaxRLPBytesPerChannelOverride *uint64 // MaxRLPBytesPerChannel override
}

func (cs *ChainSpec) MaxRLPBytesPerChannel(t uint64) uint64 {
	if o := cs.MaxRLPBytesPerChannelOverride; o != nil {
		return *o
	}
	return cs.ChainSpec.MaxRLPBytesPerChannel(t)
}
