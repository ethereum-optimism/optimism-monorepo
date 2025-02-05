package testutils

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

type MockInteropBackend struct {
	Mock mock.Mock
}

func (m *MockInteropBackend) UnsafeView(ctx context.Context, chainID eth.ChainID, unsafe types.ReferenceView) (types.ReferenceView, error) {
	result := m.Mock.MethodCalled("UnsafeView", chainID, unsafe)
	return result.Get(0).(types.ReferenceView), *result.Get(1).(*error)
}

func (m *MockInteropBackend) ExpectUnsafeView(chainID eth.ChainID, unsafe types.ReferenceView, result types.ReferenceView, err error) {
	m.Mock.On("UnsafeView", chainID, unsafe).Once().Return(result, &err)
}

func (m *MockInteropBackend) OnUnsafeView(chainID eth.ChainID, fn func(request types.ReferenceView) (result types.ReferenceView, err error)) {
	var result types.ReferenceView
	var err error
	m.Mock.On("UnsafeView", chainID, mock.Anything).Run(func(args mock.Arguments) {
		v := args[0].(types.ReferenceView)
		result, err = fn(v)
	}).Return(result, &err)
}

func (m *MockInteropBackend) SafeView(ctx context.Context, chainID eth.ChainID, safe types.ReferenceView) (types.ReferenceView, error) {
	result := m.Mock.MethodCalled("SafeView", chainID, safe)
	return result.Get(0).(types.ReferenceView), *result.Get(1).(*error)
}

func (m *MockInteropBackend) ExpectSafeView(chainID eth.ChainID, safe types.ReferenceView, result types.ReferenceView, err error) {
	m.Mock.On("SafeView", chainID, safe).Once().Return(result, &err)
}

func (m *MockInteropBackend) OnSafeView(chainID eth.ChainID, fn func(request types.ReferenceView) (result types.ReferenceView, err error)) {
	var result types.ReferenceView
	var err error
	m.Mock.On("SafeView", chainID, mock.Anything).Run(func(args mock.Arguments) {
		v := args[0].(types.ReferenceView)
		result, err = fn(v)
	}).Return(result, &err)
}

func (m *MockInteropBackend) Finalized(ctx context.Context, chainID eth.ChainID) (eth.BlockID, error) {
	result := m.Mock.MethodCalled("Finalized", chainID)
	return result.Get(0).(eth.BlockID), *result.Get(1).(*error)
}

func (m *MockInteropBackend) ExpectFinalized(chainID eth.ChainID, result eth.BlockID, err error) {
	m.Mock.On("Finalized", chainID).Once().Return(result, &err)
}

func (m *MockInteropBackend) CrossDerivedFrom(ctx context.Context, chainID eth.ChainID, derived eth.BlockID) (eth.L1BlockRef, error) {
	result := m.Mock.MethodCalled("CrossDerivedFrom", chainID, derived)
	return result.Get(0).(eth.L1BlockRef), *result.Get(1).(*error)
}

func (m *MockInteropBackend) ExpectDerivedFrom(chainID eth.ChainID, derived eth.BlockID, result eth.L1BlockRef, err error) {
	m.Mock.On("CrossDerivedFrom", chainID, derived).Once().Return(result, &err)
}

func (m *MockInteropBackend) UpdateLocalUnsafe(ctx context.Context, chainID eth.ChainID, head eth.BlockRef) error {
	result := m.Mock.MethodCalled("UpdateLocalUnsafe", chainID, head)
	return *result.Get(0).(*error)
}

func (m *MockInteropBackend) ExpectUpdateLocalUnsafe(chainID eth.ChainID, head eth.BlockRef, err error) {
	m.Mock.On("UpdateLocalUnsafe", chainID, head).Once().Return(&err)
}

func (m *MockInteropBackend) ExpectAnyUpdateLocalUnsafe(chainID eth.ChainID, err error) {
	m.Mock.On("UpdateLocalUnsafe", chainID, mock.Anything).Once().Return(&err)
}

func (m *MockInteropBackend) UpdateLocalSafe(ctx context.Context, chainID eth.ChainID, derivedFrom eth.L1BlockRef, lastDerived eth.BlockRef) error {
	result := m.Mock.MethodCalled("UpdateLocalSafe", chainID, derivedFrom, lastDerived)
	return *result.Get(0).(*error)
}

func (m *MockInteropBackend) ExpectUpdateLocalSafe(chainID eth.ChainID, derivedFrom eth.L1BlockRef, lastDerived eth.BlockRef, err error) {
	m.Mock.On("UpdateLocalSafe", chainID, derivedFrom, lastDerived).Once().Return(&err)
}

func (m *MockInteropBackend) UpdateFinalizedL1(ctx context.Context, chainID eth.ChainID, finalized eth.L1BlockRef) error {
	result := m.Mock.MethodCalled("UpdateFinalizedL1", chainID, finalized)
	return *result.Get(0).(*error)
}

func (m *MockInteropBackend) AssertExpectations(t mock.TestingT) {
	m.Mock.AssertExpectations(t)
}
