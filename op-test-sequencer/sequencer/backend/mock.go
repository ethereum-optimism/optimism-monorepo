package backend

import "context"

type MockBackend struct{}

func NewMockBackend() *MockBackend {
	return &MockBackend{}
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
