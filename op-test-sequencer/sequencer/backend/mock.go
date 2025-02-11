package backend

import (
	"context"

	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/builder"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/frontend"
)

type MockBackend struct{}

func NewMockBackend() *MockBackend {
	return &MockBackend{}
}

func (ba *MockBackend) Builder() frontend.BuildBackend {
	return builder.NoopBuilder{}
}

func (ba *MockBackend) Start(ctx context.Context) error {
	return nil
}

func (ba *MockBackend) Stop(ctx context.Context) error {
	return nil
}

func (ba *MockBackend) Hello(ctx context.Context, name string) (string, error) {
	return "hello " + name + "!", nil
}
