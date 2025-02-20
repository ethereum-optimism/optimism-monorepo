package superevents

import (
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

type ChainProcessEvent struct {
	ChainID eth.ChainID
	Target  uint64
}

func (ev ChainProcessEvent) String() string {
	return "chain-process"
}

type UpdateCrossUnsafeRequestEvent struct {
	ChainID eth.ChainID
}

func (ev UpdateCrossUnsafeRequestEvent) String() string {
	return "update-cross-unsafe-request"
}

type UpdateCrossSafeRequestEvent struct {
	ChainID eth.ChainID
}

func (ev UpdateCrossSafeRequestEvent) String() string {
	return "update-cross-safe-request"
}

type LocalUnsafeUpdateEvent struct {
	ChainID        eth.ChainID
	NewLocalUnsafe eth.BlockRef
}

func (ev LocalUnsafeUpdateEvent) String() string {
	return "local-unsafe-update"
}

type LocalSafeUpdateEvent struct {
	ChainID      eth.ChainID
	NewLocalSafe types.DerivedBlockSealPair
}

func (ev LocalSafeUpdateEvent) String() string {
	return "local-safe-update"
}

type CrossUnsafeUpdateEvent struct {
	ChainID        eth.ChainID
	NewCrossUnsafe types.BlockSeal
}

func (ev CrossUnsafeUpdateEvent) String() string {
	return "cross-unsafe-update"
}

type CrossSafeUpdateEvent struct {
	ChainID      eth.ChainID
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
	ChainID     eth.ChainID
	FinalizedL2 types.BlockSeal
}

func (ev FinalizedL2UpdateEvent) String() string {
	return "finalized-l2-update"
}

type LocalSafeOutOfSyncEvent struct {
	ChainID eth.ChainID
	L1Ref   eth.BlockRef
	Err     error
}

func (ev LocalSafeOutOfSyncEvent) String() string {
	return "local-safe-out-of-sync"
}

type LocalUnsafeReceivedEvent struct {
	ChainID        eth.ChainID
	NewLocalUnsafe eth.BlockRef
}

func (ev LocalUnsafeReceivedEvent) String() string {
	return "local-unsafe-received"
}

type LocalDerivedEvent struct {
	ChainID eth.ChainID
	Derived types.DerivedBlockRefPair
}

func (ev LocalDerivedEvent) String() string {
	return "local-derived"
}

type LocalDerivedOriginUpdateEvent struct {
	ChainID eth.ChainID
	Origin  eth.BlockRef
}

func (ev LocalDerivedOriginUpdateEvent) String() string {
	return "local-derived-origin-update"
}

type AnchorEvent struct {
	ChainID eth.ChainID
	Anchor  types.DerivedBlockRefPair
}

func (ev AnchorEvent) String() string {
	return "anchor"
}

type InvalidateLocalSafeEvent struct {
	ChainID   eth.ChainID
	Candidate types.DerivedBlockRefPair
}

func (ev InvalidateLocalSafeEvent) String() string {
	return "invalidate-local-safe"
}

type RewindL1Event struct {
	IncomingBlock eth.BlockID
}

func (ev RewindL1Event) String() string {
	return "rewind-l1"
}

type ReplaceBlockEvent struct {
	ChainID     eth.ChainID
	Replacement types.BlockReplacement
}

func (ev ReplaceBlockEvent) String() string {
	return "replace-block-event"
}

type ChainRewoundEvent struct {
	ChainID eth.ChainID
}

func (ev ChainRewoundEvent) String() string {
	return "chain-rewound"
}
