package testutils

import (
	"context"
	"reflect"

	"github.com/ethereum-optimism/optimism/op-service/client"
	"github.com/stretchr/testify/mock"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/rpc"
)

var _ client.RPC = &MockRPC{}

type MockRPC struct {
	mock.Mock
}

func (m *MockRPC) Close() {
	m.Mock.Called()
}

func (m *MockRPC) ExpectClose() {
	m.Mock.On("Close").Once().Return()
}

func (m *MockRPC) CallContext(ctx context.Context, result any, method string, args ...any) error {
	out := m.Mock.Called(ctx, result, method, args)
	return out.Error(0)
}

func (m *MockRPC) ExpectCallContext(result any, method string, args []any, err error) {
	m.Mock.On("CallContext", mock.Anything, result, method, args).Once().Return(err)
}

func (m *MockRPC) BatchCallContext(ctx context.Context, b []rpc.BatchElem) error {
	out := m.Mock.Called(ctx, b)
	return out.Error(0)
}

func (m *MockRPC) ExpectBatchCallContext(b []rpc.BatchElem, err error) {
	// Arguments are mutated directly, so replace the result as long as everything else matches
	rpcElemsMatcher := mock.MatchedBy(func(elems []rpc.BatchElem) bool {
		for i, e := range elems {
			if e.Error != b[i].Error || e.Method != b[i].Method || !reflect.DeepEqual(e.Args, b[i].Args) {
				return false
			}
		}
		return true
	})

	// Replace the Result
	m.Mock.On("BatchCallContext", mock.Anything, rpcElemsMatcher).Once().Run(func(args mock.Arguments) {
		r := args.Get(1).([]rpc.BatchElem)
		for i := 0; i < len(r); i++ {
			r[i].Result = b[i].Result
		}
	}).Return(err)
}

func (m *MockRPC) Subscribe(ctx context.Context, namespace string, channel any, args ...any) (ethereum.Subscription, error) {
	out := m.Mock.Called(ctx, namespace, channel, args)
	return out.Get(0).(ethereum.Subscription), out.Error(1)
}

func (m *MockRPC) ExpectSubscribe(namespace string, channel any, args []any, sub ethereum.Subscription, err error) {
	m.Mock.On("Subscribe", mock.Anything, namespace, channel, args).Once().Return(sub, err)
}
