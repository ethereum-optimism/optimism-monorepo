package dial

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
)

// EthClientInterface is an interface for providing an ethclient.Client
// It does not describe all of the functions an ethclient.Client has, only the ones used by callers of the L2 Providers
type EthClientInterface interface {
	BlockByNumber(ctx context.Context, number *big.Int) (*types.Block, error)
	Client() *rpc.Client

	Close()
}
