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

func (ib *InteropAPI) SubscribeUnsafeBlocks(ctx context.Context) (*gethrpc.Subscription, error) {
	return ib.backend.SubscribeUnsafeBlocks(ctx)
}

func (ib *InteropAPI) UpdateCrossUnsafe(ctx context.Context, ref eth.BlockRef) error {
	return ib.backend.UpdateCrossUnsafe(ctx, ref)
}

func (ib *InteropAPI) UpdateCrossSafe(ctx context.Context, ref eth.BlockRef, derivedFrom eth.BlockRef) error {
	return ib.backend.UpdateCrossSafe(ctx, ref, derivedFrom)
}

func (ib *InteropAPI) UpdateFinalized(ctx context.Context, ref eth.BlockRef) error {
	return ib.backend.UpdateFinalized(ctx, ref)
}

func (ib *InteropAPI) AnchorPoint(ctx context.Context) (l1, l2 eth.BlockRef, err error) {
	return ib.backend.AnchorPoint(ctx)
}

func (ib *InteropAPI) Reset(ctx context.Context, unsafe, safe, finalized eth.BlockRef) error {
	return ib.Reset(ctx, unsafe, safe, finalized)
}

func (ib *InteropAPI) TryDeriveNext(ctx context.Context, prevL2 eth.BlockRef, fromL1 eth.BlockRef) (derived eth.BlockRef, derivedFrom eth.BlockRef, err error) {
	return ib.backend.TryDeriveNext(ctx, prevL2, fromL1)
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
