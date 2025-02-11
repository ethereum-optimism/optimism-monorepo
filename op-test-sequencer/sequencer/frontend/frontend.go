package frontend

import (
	"context"

	"github.com/ethereum-optimism/optimism/op-service/eth"
)

type AdminBackend interface {
	Hello(ctx context.Context, name string) (string, error)
}

type AdminFrontend struct {
	Backend AdminBackend
}

func (af *AdminFrontend) Hello(ctx context.Context, name string) (string, error) {
	return af.Backend.Hello(ctx, name)
}

type BuildBackend interface {
	Open(ctx context.Context) (JobID, error)
	Cancel(ctx context.Context, jobID JobID) error
	Seal(ctx context.Context, jobID JobID) (eth.BlockRef, error)
}

type BuildFrontend struct {
	Backend interface {
		Builder() BuildBackend
	}
}

func (bf *BuildFrontend) Open(ctx context.Context) (JobID, error) {
	return bf.Backend.Builder().Open(ctx)
}

func (bf *BuildFrontend) Cancel(ctx context.Context, jobID JobID) error {
	return bf.Backend.Builder().Cancel(ctx, jobID)
}

func (bf *BuildFrontend) Seal(ctx context.Context, jobID JobID) (eth.BlockRef, error) {
	return bf.Backend.Builder().Seal(ctx, jobID)
}
