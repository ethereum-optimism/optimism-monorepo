package interop

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	gethrpc "github.com/ethereum/go-ethereum/rpc"

	"github.com/ethereum-optimism/optimism/op-node/rollup/event"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/rpc"
	supervisortypes "github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

// ManagedMode makes the op-node managed by an op-supervisor,
// by serving sync work and updating the canonical chain based on instructions.
type ManagedMode struct {
	log log.Logger

	emitter event.Emitter

	srv *rpc.Server
}

var _ SubSystem = (*ManagedMode)(nil)

func (s *ManagedMode) AttachEmitter(em event.Emitter) {
	s.emitter = em
}

func (s *ManagedMode) OnEvent(ev event.Event) bool {
	// TODO(#13336): let all active subscriptions now
	return false
}

func (s *ManagedMode) Start(ctx context.Context) error {
	interopAPI := &InteropAPI{}
	s.srv.AddAPI(gethrpc.API{
		Namespace:     "interop",
		Service:       interopAPI,
		Authenticated: true,
	})
	if err := s.srv.Start(); err != nil {
		return fmt.Errorf("failed to start interop RPC server: %w", err)
	}

	return nil
}

func (s *ManagedMode) Stop(ctx context.Context) error {
	// TODO(#13336): toggle closing state

	// stop RPC server
	if err := s.srv.Stop(); err != nil {
		return fmt.Errorf("failed to stop interop sub-system RPC server: %w", err)
	}

	s.log.Info("Interop sub-system stopped")
	return nil
}

type InteropAPI struct {
	// TODO(#13336): event emitter handle
	// TODO(#13336): event await util
}

func (ib *InteropAPI) SubscribeUnsafeBlocks(ctx context.Context) (*gethrpc.Subscription, error) {
	// TODO(#13336): create subscription, and get new unsafe-block events to feed into it
	return nil, nil
}

func (ib *InteropAPI) UpdateCrossUnsafe(ctx context.Context, ref eth.BlockRef) error {
	// TODO(#13336): cross-unsafe update -> fire event
	// TODO(#13336): await engine update or ctx timeout -> error maybe
	return nil
}

func (ib *InteropAPI) UpdateCrossSafe(ctx context.Context, ref eth.BlockRef) error {
	// TODO(#13336): cross-safe update -> fire event
	// TODO(#13336): await forkchoice update or ctx timeout -> error maybe
	return nil
}

func (ib *InteropAPI) UpdateFinalized(ctx context.Context, ref eth.BlockRef) error {
	// TODO(#13336): finalized update -> fire event
	// TODO(#13336): await forkchoice update or ctx timeout -> error maybe
	return nil
}

func (ib *InteropAPI) AnchorPoint(ctx context.Context) (l1, l2 eth.BlockRef, err error) {
	// TODO(#13336): return genesis anchor point from rollup config
	return
}

func (ib *InteropAPI) Reset(ctx context.Context) error {
	// TODO(#13336): fire reset event
	// TODO(#13336): await reset-confirmed event or ctx timeout
	return nil
}

func (ib *InteropAPI) TryDeriveNext(ctx context.Context, nextL1 eth.BlockRef) error {
	// TODO(#13336): fire derivation step event
	// TODO(#13336): await deriver progress (L1 or L2 kind of progress) or ctx timeout
	// TODO(#13336): need to not auto-derive the next thing until next TryDeriveNext call: need to modify driver
	// TODO(#13336): return the L1 or L2 progress
	return nil
}

func (ib *InteropAPI) FetchReceipts(ctx context.Context, blockHash common.Hash) (types.Receipts, error) {
	// TODO(#13336): use execution engine to fetch the receipts
	return nil, nil
}

func (ib *InteropAPI) BlockRefByNumber(ctx context.Context, num uint64) (eth.BlockRef, error) {
	// (#13336): use execution engine to fetch block-ref by number
	return eth.BlockRef{}, nil
}

func (ib *InteropAPI) ChainID(ctx context.Context) (supervisortypes.ChainID, error) {
	// (#13336): fetch chain ID
	return supervisortypes.ChainID{}, nil
}
