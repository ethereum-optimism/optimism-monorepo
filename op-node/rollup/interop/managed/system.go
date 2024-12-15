package managed

import (
	"context"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	gethevent "github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	gethrpc "github.com/ethereum/go-ethereum/rpc"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-node/rollup/engine"
	"github.com/ethereum-optimism/optimism/op-node/rollup/event"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/rpc"
	supervisortypes "github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

type L2Source interface {
	L2BlockRefByHash(ctx context.Context, hash common.Hash) (eth.L2BlockRef, error)
	L2BlockRefByNumber(ctx context.Context, num uint64) (eth.L2BlockRef, error)
	BlockRefByNumber(ctx context.Context, num uint64) (eth.BlockRef, error)
	FetchReceipts(ctx context.Context, blockHash common.Hash) (eth.BlockInfo, types.Receipts, error)
}

type L1Source interface {
	L1BlockRefByHash(ctx context.Context, hash common.Hash) (eth.L1BlockRef, error)
}

// ManagedMode makes the op-node managed by an op-supervisor,
// by serving sync work and updating the canonical chain based on instructions.
type ManagedMode struct {
	log log.Logger

	emitter event.Emitter

	l1 L1Source
	l2 L2Source

	unsafeBlocks gethevent.FeedOf[eth.BlockRef]

	cfg *rollup.Config

	srv       *rpc.Server
	jwtSecret eth.Bytes32
}

func NewManagedMode(log log.Logger, cfg *rollup.Config, addr string, port int, jwtSecret eth.Bytes32, l1 L1Source, l2 L2Source) *ManagedMode {
	out := &ManagedMode{
		log:       log,
		cfg:       cfg,
		l1:        l1,
		l2:        l2,
		jwtSecret: jwtSecret,
	}

	out.srv = rpc.NewServer(addr, port, "v0.0.0",
		rpc.WithWebsocketEnabled(),
		rpc.WithLogger(log),
		rpc.WithJWTSecret(jwtSecret[:]),
		rpc.WithAPIs([]gethrpc.API{
			{
				Namespace:     "interop",
				Service:       &InteropAPI{backend: out},
				Authenticated: true,
			},
		}))
	return out
}

func (m *ManagedMode) Start(ctx context.Context) error {
	if m.emitter == nil {
		return errors.New("must have emitter before starting")
	}
	if err := m.srv.Start(); err != nil {
		return fmt.Errorf("failed to start interop RPC server: %w", err)
	}
	return nil
}

func (m *ManagedMode) WSEndpoint() string {
	return fmt.Sprintf("ws://%s", m.srv.Endpoint())
}

func (m *ManagedMode) JWTSecret() eth.Bytes32 {
	return m.jwtSecret
}

func (m *ManagedMode) Stop(ctx context.Context) error {
	// stop RPC server
	if err := m.srv.Stop(); err != nil {
		return fmt.Errorf("failed to stop interop sub-system RPC server: %w", err)
	}

	m.log.Info("Interop sub-system stopped")
	return nil
}

func (m *ManagedMode) AttachEmitter(em event.Emitter) {
	m.emitter = em
}

func (m *ManagedMode) OnEvent(ev event.Event) bool {
	switch x := ev.(type) {
	case engine.UnsafeUpdateEvent:
		m.unsafeBlocks.Send(x.Ref.BlockRef())
	}
	return false
}

func (m *ManagedMode) SubscribeUnsafeBlocks(ctx context.Context) (*gethrpc.Subscription, error) {
	notifier, supported := gethrpc.NotifierFromContext(ctx)
	if !supported {
		return &gethrpc.Subscription{}, gethrpc.ErrNotificationsUnsupported
	}
	m.log.Info("Opening unsafe-blocks subscription via interop RPC")

	rpcSub := notifier.CreateSubscription()
	ch := make(chan eth.BlockRef, 10)
	unsafeBlocksSub := m.unsafeBlocks.Subscribe(ch)
	go func() {
		defer m.log.Info("Closing unsafe-blocks interop RPC subscription")
		defer unsafeBlocksSub.Unsubscribe()

		select {
		case ref := <-ch:
			if err := notifier.Notify(rpcSub.ID, ref); err != nil {
				m.log.Warn("Failed to notify RPC subscription of unsafe block", "err", err)
				return
			}
		case err := <-rpcSub.Err():
			m.log.Warn("RPC subscription for unsafe blocks failed", "err", err)
		}
	}()

	return rpcSub, nil
}

func (m *ManagedMode) UpdateCrossUnsafe(ctx context.Context, ref eth.BlockRef) error {
	l2Ref, err := m.l2.L2BlockRefByHash(ctx, ref.Hash)
	if err != nil {
		return fmt.Errorf("failed to get L2BlockRef: %w", err)
	}
	m.emitter.Emit(engine.PromoteCrossUnsafeEvent{
		Ref: l2Ref,
	})
	// We return early: there is no point waiting for the cross-unsafe engine-update synchronously.
	// All error-feedback comes to the supervisor by aborting derivation tasks with an error.
	return nil
}

func (m *ManagedMode) UpdateCrossSafe(ctx context.Context, ref eth.BlockRef, derivedFrom eth.BlockRef) error {
	l2Ref, err := m.l2.L2BlockRefByHash(ctx, ref.Hash)
	if err != nil {
		return fmt.Errorf("failed to get L2BlockRef: %w", err)
	}
	m.emitter.Emit(engine.PromoteSafeEvent{
		Ref:         l2Ref,
		DerivedFrom: derivedFrom,
	})
	// We return early: there is no point waiting for the cross-safe engine-update synchronously.
	// All error-feedback comes to the supervisor by aborting derivation tasks with an error.
	return nil
}

func (m *ManagedMode) UpdateFinalized(ctx context.Context, ref eth.BlockRef) error {
	l2Ref, err := m.l2.L2BlockRefByHash(ctx, ref.Hash)
	if err != nil {
		return fmt.Errorf("failed to get L2BlockRef: %w", err)
	}
	m.emitter.Emit(engine.PromoteFinalizedEvent{Ref: l2Ref})
	// We return early: there is no point waiting for the finalized engine-update synchronously.
	// All error-feedback comes to the supervisor by aborting derivation tasks with an error.
	return nil
}

func (m *ManagedMode) AnchorPoint(ctx context.Context) (l1, l2 eth.BlockRef, err error) {
	l1Ref, err := m.l1.L1BlockRefByHash(ctx, m.cfg.Genesis.L1.Hash)
	if err != nil {
		return eth.BlockRef{}, eth.BlockRef{}, fmt.Errorf("failed to fetch L1 block ref: %w", err)
	}
	l2Ref, err := m.l2.L2BlockRefByHash(ctx, m.cfg.Genesis.L2.Hash)
	if err != nil {
		return eth.BlockRef{}, eth.BlockRef{}, fmt.Errorf("failed to fetch L2 block ref: %w", err)
	}
	return l1Ref, l2Ref.BlockRef(), nil
}

func (m *ManagedMode) Reset(ctx context.Context, unsafe, safe, finalized eth.BlockRef) error {
	unsafeRef, err := m.l2.L2BlockRefByNumber(ctx, unsafe.Number)
	if err != nil {
		if errors.Is(err, ethereum.NotFound) {
			// TODO special error to signal to roll back more
		}
		return fmt.Errorf("unable to find reset reference point: %w", err)
	}
	if unsafeRef.Hash != unsafe.Hash {
		// TODO special error to signal to roll back more
	}

	safeRef, err := m.l2.L2BlockRefByNumber(ctx, safe.Number)
	if err != nil {
		if errors.Is(err, ethereum.NotFound) {
			// TODO special error to signal to roll back more
		}
		return fmt.Errorf("unable to find reset reference point: %w", err)
	}
	if safeRef.Hash != safe.Hash {
		// TODO special error to signal to roll back more
	}

	finalizedRef, err := m.l2.L2BlockRefByNumber(ctx, finalized.Number)
	if err != nil {
		if errors.Is(err, ethereum.NotFound) {
			// TODO special error to signal to roll back more
		}
		return fmt.Errorf("unable to find reset reference point: %w", err)
	}
	if finalizedRef.Hash != finalized.Hash {
		// TODO special error to signal to roll back more
	}

	m.emitter.Emit(engine.ForceEngineResetEvent{
		Unsafe:    unsafeRef,
		Safe:      safeRef,
		Finalized: finalizedRef,
	})
	return nil
}

func (m *ManagedMode) TryDeriveNext(ctx context.Context, prevL2 eth.BlockRef, fromL1 eth.BlockRef) (derived eth.BlockRef, derivedFrom eth.BlockRef, err error) {

	// TODO fire a derivation instruction event with (prevL2, fromL1)

	// TODO in deriver, remember the last instruction event.

	// On instruction, check prevL2, check fromL1
	// On instruction mismatch; send error
	// On L1 exhaust; mark the last instruction as done

	// TODO await a engine.LocalSafeUpdateEvent (L2 update case)
	//  or a derive.DeriverL1StatusEvent (L1 update case)
	//  or a ctx timeout

	// TODO(#13336): need to not auto-derive the next thing until next TryDeriveNext call: need to modify driver

	// Sanity check that the L1 or L2 progress is a bump by 1, if not,
	// then elsewhere we are deriving new blocks while we shouldn't.

	// TODO(#13336): return the L1 or L2 progress
	return eth.BlockRef{}, eth.BlockRef{}, nil
}

func (m *ManagedMode) FetchReceipts(ctx context.Context, blockHash common.Hash) (types.Receipts, error) {
	_, receipts, err := m.l2.FetchReceipts(ctx, blockHash)
	return receipts, err
}

func (m *ManagedMode) BlockRefByNumber(ctx context.Context, num uint64) (eth.BlockRef, error) {
	return m.l2.BlockRefByNumber(ctx, num)
}

func (m *ManagedMode) ChainID(ctx context.Context) (supervisortypes.ChainID, error) {
	return supervisortypes.ChainIDFromBig(m.cfg.L2ChainID), nil
}
