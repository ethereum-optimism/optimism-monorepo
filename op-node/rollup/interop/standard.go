package interop

import (
	"context"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-node/rollup/event"
	"github.com/ethereum-optimism/optimism/op-service/sources"
)

// StandardMode makes the op-node follow the canonical chain based on a read-only supervisor endpoint.
type StandardMode struct {
	log log.Logger

	emitter event.Emitter

	cl *sources.SupervisorClient
}

var _ SubSystem = (*StandardMode)(nil)

func (s *StandardMode) AttachEmitter(em event.Emitter) {
	s.emitter = em
}

func (s *StandardMode) OnEvent(ev event.Event) bool {
	// TODO(#13337): hook up to existing interop deriver
	return false
}

func (s *StandardMode) Start(ctx context.Context) error {
	s.log.Info("Interop sub-system started in follow-mode")
	return nil
}

func (s *StandardMode) Stop(ctx context.Context) error {
	// TODO(#13337) toggle closing state

	s.log.Info("Interop sub-system stopped")
	return s.cl.Stop(ctx)
}
