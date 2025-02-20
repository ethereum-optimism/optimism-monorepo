package syncnode

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

type SyncNodeCollection interface {
	Load(ctx context.Context, logger log.Logger) ([]SyncNodeSetup, error)
	Check() error
}

type SyncNodeSetup interface {
	Setup(ctx context.Context, logger log.Logger) (SyncNode, error)
}

type SyncSource interface {
	BlockRefByNumber(ctx context.Context, number uint64) (eth.BlockRef, error)
	FetchReceipts(ctx context.Context, blockHash common.Hash) (gethtypes.Receipts, error)
	ChainID(ctx context.Context) (eth.ChainID, error)
	OutputV0AtTimestamp(ctx context.Context, timestamp uint64) (*eth.OutputV0, error)
	PendingOutputV0AtTimestamp(ctx context.Context, timestamp uint64) (*eth.OutputV0, error)
	L2BlockRefByTimestamp(ctx context.Context, timestamp uint64) (eth.L2BlockRef, error)
	// String identifies the sync source
	String() string
}

type SyncControl interface {
	SubscribeEvents(ctx context.Context, c chan *types.ManagedEvent) (ethereum.Subscription, error)
	PullEvent(ctx context.Context) (*types.ManagedEvent, error)

	UpdateCrossUnsafe(ctx context.Context, id eth.BlockID) error
	UpdateCrossSafe(ctx context.Context, derived eth.BlockID, source eth.BlockID) error
	UpdateFinalized(ctx context.Context, id eth.BlockID) error

	InvalidateBlock(ctx context.Context, seal types.BlockSeal) error

	Reset(ctx context.Context, unsafe, safe, finalized eth.BlockID) error
	ProvideL1(ctx context.Context, nextL1 eth.BlockRef) error
	AnchorPoint(ctx context.Context) (types.DerivedBlockRefPair, error)

	fmt.Stringer
}

type SyncNode interface {
	SyncSource
	SyncControl
}

type Node interface {
	PullEvents(ctx context.Context) (pulledAny bool, err error)
}
