package frontend

import (
	"context"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
	"github.com/ethereum/go-ethereum/common"
)

type AdminBackend interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	AddL2RPC(ctx context.Context, rpc string, jwtSecret eth.Bytes32) error
}

type QueryBackend interface {
	CheckMessage(identifier types.Identifier, payloadHash common.Hash) (types.SafetyLevel, error)
	CheckMessages(messages []types.Message, minSafety types.SafetyLevel) error
	CrossDerivedFrom(ctx context.Context, chainID types.ChainID, derived eth.BlockID) (derivedFrom eth.BlockRef, err error)
	LocalUnsafe(ctx context.Context, chainID types.ChainID) (eth.BlockID, error)
	CrossSafe(ctx context.Context, chainID types.ChainID) (types.DerivedIDPair, error)
	Finalized(ctx context.Context, chainID types.ChainID) (eth.BlockID, error)
	FinalizedL1() eth.BlockRef
}

type Backend interface {
	AdminBackend
	QueryBackend
}

type QueryFrontend struct {
	Supervisor QueryBackend
}

var _ QueryBackend = (*QueryFrontend)(nil)

// CheckMessage checks the safety-level of an individual message.
// The payloadHash references the hash of the message-payload of the message.
func (q *QueryFrontend) CheckMessage(identifier types.Identifier, payloadHash common.Hash) (types.SafetyLevel, error) {
	return q.Supervisor.CheckMessage(identifier, payloadHash)
}

// CheckMessages checks the safety-level of a collection of messages,
// and returns if the minimum safety-level is met for all messages.
func (q *QueryFrontend) CheckMessages(
	messages []types.Message,
	minSafety types.SafetyLevel) error {
	return q.Supervisor.CheckMessages(messages, minSafety)
}

func (q *QueryFrontend) LocalUnsafe(ctx context.Context, chainID types.ChainID) (eth.BlockID, error) {
	return q.Supervisor.LocalUnsafe(ctx, chainID)
}

func (q *QueryFrontend) CrossSafe(ctx context.Context, chainID types.ChainID) (types.DerivedIDPair, error) {
	return q.Supervisor.CrossSafe(ctx, chainID)
}

func (q *QueryFrontend) Finalized(ctx context.Context, chainID types.ChainID) (eth.BlockID, error) {
	return q.Supervisor.Finalized(ctx, chainID)
}

func (q *QueryFrontend) FinalizedL1() eth.BlockRef {
	return q.Supervisor.FinalizedL1()
}

func (q *QueryFrontend) CrossDerivedFrom(ctx context.Context, chainID types.ChainID, derived eth.BlockID) (derivedFrom eth.BlockRef, err error) {
	return q.Supervisor.CrossDerivedFrom(ctx, chainID, derived)
}

type AdminFrontend struct {
	Supervisor Backend
}

var _ AdminBackend = (*AdminFrontend)(nil)

// Start starts the service, if it was previously stopped.
func (a *AdminFrontend) Start(ctx context.Context) error {
	return a.Supervisor.Start(ctx)
}

// Stop stops the service, if it was previously started.
func (a *AdminFrontend) Stop(ctx context.Context) error {
	return a.Supervisor.Stop(ctx)
}

// AddL2RPC adds a new L2 chain to the supervisor backend
func (a *AdminFrontend) AddL2RPC(ctx context.Context, rpc string, jwtSecret eth.Bytes32) error {
	return a.Supervisor.AddL2RPC(ctx, rpc, jwtSecret)
}
