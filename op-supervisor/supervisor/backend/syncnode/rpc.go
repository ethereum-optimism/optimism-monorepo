package syncnode

import (
	"context"
	"errors"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/ethereum-optimism/optimism/op-service/client"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

type RPCSyncNode struct {
	name string
	cl   client.RPC
}

func NewRPCSyncNode(name string, cl client.RPC) *RPCSyncNode {
	return &RPCSyncNode{
		name: name,
		cl:   cl,
	}
}

var _ SyncSource = (*RPCSyncNode)(nil)
var _ SyncControl = (*RPCSyncNode)(nil)
var _ SyncNode = (*RPCSyncNode)(nil)

func (rs *RPCSyncNode) BlockRefByNumber(ctx context.Context, number uint64) (eth.BlockRef, error) {
	var out *eth.BlockRef
	err := rs.cl.CallContext(ctx, &out, "interop_blockRefByNumber", number)
	if err != nil {
		var jsonErr rpc.Error
		if errors.As(err, &jsonErr) {
			if jsonErr.ErrorCode() == 0 { // TODO
				return eth.BlockRef{}, ethereum.NotFound
			}
		}
		return eth.BlockRef{}, err
	}
	return *out, nil
}

func (rs *RPCSyncNode) FetchReceipts(ctx context.Context, blockHash common.Hash) (gethtypes.Receipts, error) {
	var out gethtypes.Receipts
	err := rs.cl.CallContext(ctx, &out, "interop_fetchReceipts", blockHash)
	if err != nil {
		var jsonErr rpc.Error
		if errors.As(err, &jsonErr) {
			if jsonErr.ErrorCode() == 0 { // TODO
				return nil, ethereum.NotFound
			}
		}
		return nil, err
	}
	return out, nil
}

func (rs *RPCSyncNode) ChainID(ctx context.Context) (types.ChainID, error) {
	var chainID types.ChainID
	err := rs.cl.CallContext(ctx, &chainID, "interop_chainID")
	return chainID, err
}

func (rs *RPCSyncNode) String() string {
	return rs.name
}

func (rs *RPCSyncNode) SubscribeResetEvents(ctx context.Context, dest chan string) (ethereum.Subscription, error) {
	return rs.cl.Subscribe(ctx, "interop", dest, "resetEvents")
}

func (rs *RPCSyncNode) SubscribeUnsafeBlocks(ctx context.Context, dest chan eth.BlockRef) (ethereum.Subscription, error) {
	return rs.cl.Subscribe(ctx, "interop", dest, "unsafeBlocks")
}

func (rs *RPCSyncNode) SubscribeDerivationUpdates(ctx context.Context, dest chan types.DerivedPair) (ethereum.Subscription, error) {
	return rs.cl.Subscribe(ctx, "interop", dest, "derivationUpdates")
}

func (rs *RPCSyncNode) SubscribeExhaustL1Events(ctx context.Context, dest chan types.DerivedPair) (ethereum.Subscription, error) {
	return rs.cl.Subscribe(ctx, "interop", dest, "exhaustL1Events")
}

func (rs *RPCSyncNode) UpdateCrossUnsafe(ctx context.Context, id eth.BlockID) error {
	return rs.cl.CallContext(ctx, nil, "interop_updateCrossUnsafe", id)
}

func (rs *RPCSyncNode) UpdateCrossSafe(ctx context.Context, derived eth.BlockID, derivedFrom eth.BlockID) error {
	return rs.cl.CallContext(ctx, nil, "interop_updateCrossSafe", derived, derivedFrom)
}

func (rs *RPCSyncNode) UpdateFinalized(ctx context.Context, id eth.BlockID) error {
	return rs.cl.CallContext(ctx, nil, "interop_updateFinalized", id)
}

func (rs *RPCSyncNode) Reset(ctx context.Context, unsafe, safe, finalized eth.BlockID) error {
	return rs.cl.CallContext(ctx, nil, "interop_reset", unsafe, safe, finalized)
}

func (rs *RPCSyncNode) ProvideL1(ctx context.Context, nextL1 eth.BlockRef) error {
	return rs.cl.CallContext(ctx, nil, "interop_provideL1", nextL1)
}

func (rs *RPCSyncNode) AnchorPoint(ctx context.Context) (types.DerivedPair, error) {
	var out types.DerivedPair
	err := rs.cl.CallContext(ctx, &out, "interop_anchorPoint")
	return out, err
}
