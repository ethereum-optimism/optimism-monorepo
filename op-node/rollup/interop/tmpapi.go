package interop

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
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
	interopAPI := &TemporaryInteropAPI{Eng: eng}

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
	return fmt.Sprintf("http://%s", s.srv.Endpoint())
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
	Eng Engine
}

func (ib *TemporaryInteropAPI) FetchReceipts(ctx context.Context, blockHash common.Hash) (types.Receipts, error) {
	_, receipts, err := ib.Eng.FetchReceipts(ctx, blockHash)
	return receipts, err
}

func (ib *TemporaryInteropAPI) BlockRefByNumber(ctx context.Context, num uint64) (eth.BlockRef, error) {
	return ib.Eng.BlockRefByNumber(ctx, num)
}

func (ib *TemporaryInteropAPI) ChainID(ctx context.Context) (supervisortypes.ChainID, error) {
	v, err := ib.Eng.ChainID(ctx)
	if err != nil {
		return supervisortypes.ChainID{}, err
	}
	return supervisortypes.ChainIDFromBig(v), nil
}
