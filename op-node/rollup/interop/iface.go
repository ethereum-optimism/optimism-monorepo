package interop

import (
	"context"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-node/rollup/event"
	"github.com/ethereum-optimism/optimism/op-service/sources"
)

type SubSystem interface {
	event.Deriver
	event.AttachEmitter
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

type Setup interface {
	Setup(ctx context.Context, logger log.Logger) (SubSystem, error)
	TemporarySetup(ctx context.Context, logger log.Logger, eng Engine) (
		*sources.SupervisorClient, *TemporaryInteropServer, error)
	Check() error
}
