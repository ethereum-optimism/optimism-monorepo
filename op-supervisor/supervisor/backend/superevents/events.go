package superevents

import (
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

type ChainProcessEvent struct {
	ChainID types.ChainID
	Target  uint64
}

func (ev ChainProcessEvent) String() string {
	return "chain-process"
}

type UpdateCrossUnsafeRequestEvent struct {
	ChainID types.ChainID
}

func (ev UpdateCrossUnsafeRequestEvent) String() string {
	return "update-cross-unsafe-request"
}

type UpdateCrossSafeRequestEvent struct {
	ChainID types.ChainID
}

func (ev UpdateCrossSafeRequestEvent) String() string {
	return "update-cross-safe-request"
}

type LocalUnsafeUpdateEvent struct {
	ChainID        types.ChainID
	NewLocalUnsafe eth.BlockRef
}

func (ev LocalUnsafeUpdateEvent) String() string {
	return "local-unsafe-update"
}

type LocalSafeUpdateEvent struct {
	ChainID      types.ChainID
	NewLocalSafe types.DerivedBlockSealPair
}

func (ev LocalSafeUpdateEvent) String() string {
	return "local-safe-update"
}

type CrossUnsafeUpdateEvent struct {
	ChainID        types.ChainID
	NewCrossUnsafe types.BlockSeal
}

func (ev CrossUnsafeUpdateEvent) String() string {
	return "cross-unsafe-update"
}

type CrossSafeUpdateEvent struct {
	ChainID      types.ChainID
	NewCrossSafe types.DerivedBlockSealPair
}

func (ev CrossSafeUpdateEvent) String() string {
	return "cross-safe-update"
}

type FinalizedL1RequestEvent struct {
	FinalizedL1 eth.BlockRef
}

func (ev FinalizedL1RequestEvent) String() string {
	return "finalized-l1-request"
}

type FinalizedL1UpdateEvent struct {
	FinalizedL1 eth.BlockRef
}

func (ev FinalizedL1UpdateEvent) String() string {
	return "finalized-l1-update"
}

type FinalizedL2UpdateEvent struct {
	ChainID     types.ChainID
	FinalizedL2 types.BlockSeal
}

func (ev FinalizedL2UpdateEvent) String() string {
	return "finalized-l2-update"
}

type LocalUnsafeReceivedEvent struct {
	ChainID        types.ChainID
	NewLocalUnsafe eth.BlockRef
}

func (ev LocalUnsafeReceivedEvent) String() string {
	return "local-unsafe-received"
}

type LocalDerivedEvent struct {
	ChainID types.ChainID
	Derived types.DerivedBlockRefPair
}

func (ev LocalDerivedEvent) String() string {
	return "local-derived"
}

type AnchorEvent struct {
	ChainID types.ChainID
	Anchor  types.DerivedBlockRefPair
}

func (ev AnchorEvent) String() string {
	return "anchor"
}
