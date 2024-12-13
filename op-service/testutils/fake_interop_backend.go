package testutils

import (
	"context"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

type FakeInteropBackend struct {
	UnsafeViewFn        func(ctx context.Context, chainID types.ChainID, unsafe types.ReferenceView) (types.ReferenceView, error)
	SafeViewFn          func(ctx context.Context, chainID types.ChainID, safe types.ReferenceView) (types.ReferenceView, error)
	FinalizedFn         func(ctx context.Context, chainID types.ChainID) (eth.BlockID, error)
	DerivedFromFn       func(ctx context.Context, chainID types.ChainID, derived eth.BlockID) (eth.L1BlockRef, error)
	UpdateLocalUnsafeFn func(ctx context.Context, chainID types.ChainID, head eth.BlockRef) error
	UpdateLocalSafeFn   func(ctx context.Context, chainID types.ChainID, derivedFrom eth.L1BlockRef, lastDerived eth.BlockRef) error
	UpdateFinalizedL1Fn func(ctx context.Context, chainID types.ChainID, finalized eth.L1BlockRef) error
}

func (m *FakeInteropBackend) UnsafeView(ctx context.Context, chainID types.ChainID, unsafe types.ReferenceView) (types.ReferenceView, error) {
	return m.UnsafeViewFn(ctx, chainID, unsafe)
}

func (m *FakeInteropBackend) SafeView(ctx context.Context, chainID types.ChainID, safe types.ReferenceView) (types.ReferenceView, error) {
	return m.SafeViewFn(ctx, chainID, safe)
}

func (m *FakeInteropBackend) Finalized(ctx context.Context, chainID types.ChainID) (eth.BlockID, error) {
	return m.FinalizedFn(ctx, chainID)
}

func (m *FakeInteropBackend) CrossDerivedFrom(ctx context.Context, chainID types.ChainID, derived eth.BlockID) (eth.L1BlockRef, error) {
	return m.DerivedFromFn(ctx, chainID, derived)
}

func (m *FakeInteropBackend) UpdateLocalUnsafe(ctx context.Context, chainID types.ChainID, head eth.BlockRef) error {
	return m.UpdateLocalUnsafeFn(ctx, chainID, head)
}

func (m *FakeInteropBackend) UpdateLocalSafe(ctx context.Context, chainID types.ChainID, derivedFrom eth.L1BlockRef, lastDerived eth.BlockRef) error {
	return m.UpdateLocalSafeFn(ctx, chainID, derivedFrom, lastDerived)
}

func (m *FakeInteropBackend) UpdateFinalizedL1(ctx context.Context, chainID types.ChainID, finalized eth.L1BlockRef) error {
	return m.UpdateFinalizedL1Fn(ctx, chainID, finalized)
}
