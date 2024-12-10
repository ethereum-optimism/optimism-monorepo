package interop

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	gethrpc "github.com/ethereum/go-ethereum/rpc"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/rpc"
	supervisortypes "github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

// TemporaryInteropServer is a work-around to serve the "managed"-
// mode endpoints used by the op-supervisor for data,
// while still using the old interop deriver for syncing.
type TemporaryInteropServer struct {
	srv *rpc.Server
}

func NewTemporaryInteropServer(host string, port int, eng Engine) *TemporaryInteropServer {
	interopAPI := &TemporaryInteropAPI{eng: eng}

	srv := rpc.NewServer(host, port, "v0.0.1",
		rpc.WithAPIs([]gethrpc.API{
			{
				Namespace:     "interop",
				Service:       interopAPI,
				Authenticated: false,
			},
		}))

	return &TemporaryInteropServer{srv: srv}
}

func (s *TemporaryInteropServer) Start() error {
	return s.srv.Start()
}

func (s *TemporaryInteropServer) Endpoint() string {
	return s.srv.Endpoint()
}

func (s *TemporaryInteropServer) Close() error {
	return s.srv.Stop()
}

type Engine interface {
	FetchReceipts(ctx context.Context, blockHash common.Hash) (eth.BlockInfo, types.Receipts, error)
	BlockRefByNumber(ctx context.Context, num uint64) (eth.BlockRef, error)
	ChainID(ctx context.Context) (*big.Int, error)
}

type TemporaryInteropAPI struct {
	eng Engine
}

func (ib *TemporaryInteropAPI) FetchReceipts(ctx context.Context, id eth.BlockID) (types.Receipts, error) {
	_, receipts, err := ib.eng.FetchReceipts(ctx, id.Hash)
	return receipts, err
}

func (ib *TemporaryInteropAPI) BlockRefByNumber(ctx context.Context, num hexutil.Uint64) (eth.BlockRef, error) {
	return ib.eng.BlockRefByNumber(ctx, uint64(num))
}

func (ib *TemporaryInteropAPI) ChainID(ctx context.Context) (supervisortypes.ChainID, error) {
	v, err := ib.eng.ChainID(ctx)
	if err != nil {
		return supervisortypes.ChainID{}, err
	}
	return supervisortypes.ChainIDFromBig(v), nil
}
