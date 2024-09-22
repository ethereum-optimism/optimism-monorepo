package conductor

import (
	"context"

	"github.com/ethereum-optimism/optimism/op-service/eth"
)

// SequencerConductor is an interface for the driver to communicate with the sequencer conductor.
// It is used to determine if the current node is the active sequencer, and to commit unsafe payloads to the conductor log.
type SequencerConductor interface {
	// Initialize initializes the conductor client, make sure the remote service is reachable.
	Initialize(ctx context.Context) error
	// Leader returns true if this node is the leader sequencer.
	Leader(ctx context.Context) (bool, error)
	// CommitUnsafePayload commits an unsafe payload to the conductor FSM.
	CommitUnsafePayload(ctx context.Context, payload *eth.ExecutionPayloadEnvelope) error
	// OverrideLeader forces current node to be considered leader and be able to start sequencing during disaster situations in HA mode.
	OverrideLeader(ctx context.Context) error
	// Close closes the conductor client.
	Close()
}

// NoOpConductor is a no-op conductor that assumes this node is the leader sequencer.
type NoOpConductor struct{}

var _ SequencerConductor = &NoOpConductor{}

// Initialize returns nil if remote is reachable. NoOpConductor always returns nil.
func (c *NoOpConductor) Initialize(ctx context.Context) error {
	return nil
}

// Leader returns true if this node is the leader sequencer. NoOpConductor always returns true.
func (c *NoOpConductor) Leader(ctx context.Context) (bool, error) {
	return true, nil
}

// CommitUnsafePayload commits an unsafe payload to the conductor log.
func (c *NoOpConductor) CommitUnsafePayload(ctx context.Context, payload *eth.ExecutionPayloadEnvelope) error {
	return nil
}

// OverrideLeader implements SequencerConductor.
func (c *NoOpConductor) OverrideLeader(ctx context.Context) error {
	return nil
}

// Close closes the conductor client.
func (c *NoOpConductor) Close() {}
