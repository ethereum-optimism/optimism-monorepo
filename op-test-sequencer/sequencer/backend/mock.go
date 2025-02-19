package backend

import (
	"context"

	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work/builders/noopbuilder"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/frontend"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/seqtypes"
)

type MockBackend struct{}

var _ frontend.BuildBackend = (*MockBackend)(nil)
var _ frontend.AdminBackend = (*MockBackend)(nil)

func NewMockBackend() *MockBackend {
	return &MockBackend{}
}

func (ba *MockBackend) CreateJob(ctx context.Context, id seqtypes.BuilderID, opts *seqtypes.BuildOpts) (work.BuildJob, error) {
	return nil, noopbuilder.ErrNoBuild
}

func (ba *MockBackend) GetJob(id seqtypes.BuildJobID) work.BuildJob {
	return nil
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
