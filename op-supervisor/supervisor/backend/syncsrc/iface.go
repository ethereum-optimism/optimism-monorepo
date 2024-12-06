package syncsrc

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

type SyncSourceCollection interface {
	Load(ctx context.Context, logger log.Logger) ([]SyncSourceSetup, error)
	Check() error
}

type SyncSourceSetup interface {
	Setup(ctx context.Context, logger log.Logger) (SyncSource, error)
}

type SyncSource interface {
	BlockRefByNumber(ctx context.Context, number uint64) (eth.BlockRef, error)
	FetchReceipts(ctx context.Context, blockHash common.Hash) (gethtypes.Receipts, error)
	ChainID(ctx context.Context) (types.ChainID, error)
	// String identifies the sync source
	String() string
}
