package managed

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	gethrpc "github.com/ethereum/go-ethereum/rpc"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	supervisortypes "github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

type InteropAPI struct {
	backend *ManagedMode
}

func (ib *InteropAPI) ResetEvents(ctx context.Context) (*gethrpc.Subscription, error) {
	return ib.backend.ResetEvents(ctx)
}

func (ib *InteropAPI) UnsafeBlocks(ctx context.Context) (*gethrpc.Subscription, error) {
	return ib.backend.UnsafeBlocks(ctx)
}

func (m *InteropAPI) DerivationUpdates(ctx context.Context) (*gethrpc.Subscription, error) {
	return m.backend.DerivationUpdates(ctx)
}

func (m *InteropAPI) ExhaustL1Events(ctx context.Context) (*gethrpc.Subscription, error) {
	return m.backend.ExhaustL1Events(ctx)
}

func (ib *InteropAPI) UpdateCrossUnsafe(ctx context.Context, id eth.BlockID) error {
	return ib.backend.UpdateCrossUnsafe(ctx, id)
}

func (ib *InteropAPI) UpdateCrossSafe(ctx context.Context, derived eth.BlockID, derivedFrom eth.BlockID) error {
	return ib.backend.UpdateCrossSafe(ctx, derived, derivedFrom)
}

func (ib *InteropAPI) UpdateFinalized(ctx context.Context, id eth.BlockID) error {
	return ib.backend.UpdateFinalized(ctx, id)
}

func (ib *InteropAPI) AnchorPoint(ctx context.Context) (supervisortypes.DerivedPair, error) {
	return ib.backend.AnchorPoint(ctx)
}

func (ib *InteropAPI) Reset(ctx context.Context, unsafe, safe, finalized eth.BlockID) error {
	return ib.Reset(ctx, unsafe, safe, finalized)
}

func (ib *InteropAPI) FetchReceipts(ctx context.Context, blockHash common.Hash) (types.Receipts, error) {
	return ib.backend.FetchReceipts(ctx, blockHash)
}

func (ib *InteropAPI) BlockRefByNumber(ctx context.Context, num uint64) (eth.BlockRef, error) {
	return ib.backend.BlockRefByNumber(ctx, num)
}

func (ib *InteropAPI) ChainID(ctx context.Context) (supervisortypes.ChainID, error) {
	return ib.backend.ChainID(ctx)
}
