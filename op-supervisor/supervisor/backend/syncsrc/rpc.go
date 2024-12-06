package syncsrc

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

type RPCSyncSource struct {
	name string
	cl   client.RPC
}

var _ SyncSource = (*RPCSyncSource)(nil)

func (rs *RPCSyncSource) BlockRefByNumber(ctx context.Context, number uint64) (eth.BlockRef, error) {
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

func (rs *RPCSyncSource) FetchReceipts(ctx context.Context, blockHash common.Hash) (gethtypes.Receipts, error) {
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

func (rs *RPCSyncSource) ChainID(ctx context.Context) (types.ChainID, error) {
	var chainID types.ChainID
	err := rs.cl.CallContext(ctx, &chainID, "interop_chainID")
	return chainID, err
}

func (rs *RPCSyncSource) String() string {
	return rs.name
}
