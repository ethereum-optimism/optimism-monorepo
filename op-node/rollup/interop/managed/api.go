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

func (ib *InteropAPI) PullEvent() (*supervisortypes.ManagedEvent, error) {
	return ib.backend.PullEvent()
}

func (ib *InteropAPI) Events(ctx context.Context) (*gethrpc.Subscription, error) {
	return ib.backend.Events(ctx)
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

func (ib *InteropAPI) InvalidateBlock(ctx context.Context, seal supervisortypes.BlockSeal) error {
	return ib.backend.InvalidateBlock(ctx, seal)
}

func (ib *InteropAPI) AnchorPoint(ctx context.Context) (supervisortypes.DerivedBlockRefPair, error) {
	return ib.backend.AnchorPoint(ctx)
}

func (ib *InteropAPI) Reset(ctx context.Context, unsafe, safe, finalized eth.BlockID) error {
	return ib.backend.Reset(ctx, unsafe, safe, finalized)
}

func (ib *InteropAPI) FetchReceipts(ctx context.Context, blockHash common.Hash) (types.Receipts, error) {
	return ib.backend.FetchReceipts(ctx, blockHash)
}

func (ib *InteropAPI) BlockRefByNumber(ctx context.Context, num uint64) (eth.BlockRef, error) {
	return ib.backend.BlockRefByNumber(ctx, num)
}

func (ib *InteropAPI) ChainID(ctx context.Context) (eth.ChainID, error) {
	return ib.backend.ChainID(ctx)
}

func (ib *InteropAPI) OutputV0AtTimestamp(ctx context.Context, timestamp uint64) (*eth.OutputV0, error) {
	return ib.backend.OutputV0AtTimestamp(ctx, timestamp)
}

func (ib *InteropAPI) PendingOutputV0AtTimestamp(ctx context.Context, timestamp uint64) (*eth.OutputV0, error) {
	return ib.backend.PendingOutputV0AtTimestamp(ctx, timestamp)
}

func (ib *InteropAPI) L2BlockRefByTimestamp(ctx context.Context, timestamp uint64) (eth.L2BlockRef, error) {
	return ib.backend.L2BlockRefByTimestamp(ctx, timestamp)
}

func (ib *InteropAPI) ProvideL1(ctx context.Context, nextL1 eth.BlockRef) error {
	return ib.backend.ProvideL1(ctx, nextL1)
}
