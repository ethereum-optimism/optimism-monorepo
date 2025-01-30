package standard

import (
	"context"
	"sync"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-node/rollup/event"
	"github.com/ethereum-optimism/optimism/op-service/sources"
)

// StandardMode makes the op-node follow the canonical chain based on a read-only supervisor endpoint.
type StandardMode struct {
	log log.Logger

	emitter event.Emitter

	cl *sources.SupervisorClient

	mu sync.RWMutex
}

func NewStandardMode(log log.Logger, cl *sources.SupervisorClient) *StandardMode {
	return &StandardMode{
		log:     log,
		emitter: nil,
		cl:      cl,
	}
}

func (s *StandardMode) AttachEmitter(em event.Emitter) {
	s.emitter = em
}

func (s *StandardMode) OnEvent(ev event.Event) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch x := ev.(type) {
	case rollup.ResetEvent:
		s.log.Error("todo: interop needs to handle resets", x.Err)
		// TODO(#13337): on reset: consolidate L2 against supervisor, then do force-reset
	}
	return false
}

func (s *StandardMode) Start(ctx context.Context) error {
	s.log.Info("Interop sub-system started in follow-mode")

	// TODO(#13337): Interop standard mode implementation.
	// Poll supervisor:
	// - finalized L2 -> check if cross-safe, apply
	// - cross-safe l2 -> check if local-safe, apply
	// - cross-unsafe l2 -> check if local-unsafe, apply
	//
	// Make the polling manually triggerable. Or maybe just instantiate
	// a loop that optionally fires events to the checking part?

	return nil
}

func (s *StandardMode) Stop(ctx context.Context) error {
	// TODO(#13337) toggle closing state

	s.log.Info("Interop sub-system stopped")
	return s.cl.Stop(ctx)
}
