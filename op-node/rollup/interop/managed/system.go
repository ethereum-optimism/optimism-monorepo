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
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
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

	unsafeBlocks      gethevent.FeedOf[eth.BlockRef]
	derivationUpdates gethevent.FeedOf[supervisortypes.DerivedPair]
	exhaustL1Events   gethevent.FeedOf[supervisortypes.DerivedPair]

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
	case engine.LocalSafeUpdateEvent:
		m.derivationUpdates.Send(supervisortypes.DerivedPair{
			DerivedFrom: x.DerivedFrom,
			Derived:     x.Ref.BlockRef(),
		})
	case derive.DeriverL1StatusEvent:
		m.derivationUpdates.Send(supervisortypes.DerivedPair{
			DerivedFrom: x.Origin,
			Derived:     x.LastL2.BlockRef(),
		})
	case derive.ExhaustedL1Event:
		m.exhaustL1Events.Send(supervisortypes.DerivedPair{
			DerivedFrom: x.L1Ref,
			Derived:     x.LastL2.BlockRef(),
		})
	}
	return false
}

func (m *ManagedMode) SubscribeUnsafeBlocks(ctx context.Context) (*gethrpc.Subscription, error) {
	return rpc.SubscribeRPC(ctx, m.log.New("subscription", "unsafeBlocks"), &m.unsafeBlocks)
}

func (m *ManagedMode) SubscribeDerivationUpdates(ctx context.Context) (*gethrpc.Subscription, error) {
	return rpc.SubscribeRPC(ctx, m.log.New("subscription", "derivationUpdates"), &m.derivationUpdates)
}

func (m *ManagedMode) SubscribeExhaustL1Events(ctx context.Context) (*gethrpc.Subscription, error) {
	return rpc.SubscribeRPC(ctx, m.log.New("subscription", "exhaustL1Events"), &m.exhaustL1Events)
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

func (m *ManagedMode) AnchorPoint(ctx context.Context) (supervisortypes.DerivedPair, error) {
	l1Ref, err := m.l1.L1BlockRefByHash(ctx, m.cfg.Genesis.L1.Hash)
	if err != nil {
		return supervisortypes.DerivedPair{}, fmt.Errorf("failed to fetch L1 block ref: %w", err)
	}
	l2Ref, err := m.l2.L2BlockRefByHash(ctx, m.cfg.Genesis.L2.Hash)
	if err != nil {
		return supervisortypes.DerivedPair{}, fmt.Errorf("failed to fetch L2 block ref: %w", err)
	}
	return supervisortypes.DerivedPair{
		DerivedFrom: l1Ref,
		Derived:     l2Ref.BlockRef(),
	}, nil
}

const (
	InternalErrorRPCErrcode    = -32603
	BlockNotFoundRPCErrCode    = -39001
	ConflictingBlockRPCErrCode = -39002
)

func (m *ManagedMode) Reset(ctx context.Context, unsafe, safe, finalized eth.BlockID) error {
	logger := m.log.New("unsafe", unsafe, "safe", safe, "finalized", finalized)

	verify := func(ref eth.BlockID, name string) (eth.L2BlockRef, error) {
		result, err := m.l2.L2BlockRefByNumber(ctx, ref.Number)
		if err != nil {
			if errors.Is(err, ethereum.NotFound) {
				logger.Warn("Cannot reset, reset-anchor not found", "refName", name)
				return eth.L2BlockRef{}, &gethrpc.JsonError{
					Code:    BlockNotFoundRPCErrCode,
					Message: "Block not found",
					Data:    nil,
				}
			}
			logger.Warn("unable to find reference", "refName", name)
			return eth.L2BlockRef{}, &gethrpc.JsonError{
				Code:    InternalErrorRPCErrcode,
				Message: "failed to find block reference",
				Data:    name,
			}
		}
		if result.Hash != unsafe.Hash {
			return eth.L2BlockRef{}, &gethrpc.JsonError{
				Code:    ConflictingBlockRPCErrCode,
				Message: "Conflicting block",
				Data:    result,
			}
		}
		return result, nil
	}

	unsafeRef, err := verify(unsafe, "unsafe")
	if err != nil {
		return err
	}
	safeRef, err := verify(unsafe, "safe")
	if err != nil {
		return err
	}
	finalizedRef, err := verify(unsafe, "finalized")
	if err != nil {
		return err
	}

	m.emitter.Emit(engine.ForceEngineResetEvent{
		Unsafe:    unsafeRef,
		Safe:      safeRef,
		Finalized: finalizedRef,
	})
	return nil
}

func (m *ManagedMode) ProvideL1(ctx context.Context, fromL1 eth.BlockRef) error {
	// TODO: when op-node is in need of a next L1 block (it tells through L1 exhaust eventS),
	// the supervisor can provide it with this method.
	// Here we then need to fire an event, which the L1-traversal can pick up to unblock itself.
	// And send an error event maybe if the provided L1 block does not fit on top of the last known L1 block.
	return nil
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
