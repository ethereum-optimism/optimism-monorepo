package sources

import (
	"context"
	"fmt"

	"github.com/ethereum-optimism/optimism/op-service/client"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
	"github.com/ethereum/go-ethereum/common"
)

type SupervisorClient struct {
	client client.RPC
}

func NewSupervisorClient(client client.RPC) *SupervisorClient {
	return &SupervisorClient{
		client: client,
	}
}

func (cl *SupervisorClient) Stop(ctx context.Context) error {
	var result error
	err := cl.client.CallContext(
		ctx,
		&result,
		"admin_stop")
	if err != nil {
		return fmt.Errorf("failed to stop Supervisor: %w", err)
	}
	return result
}

func (cl *SupervisorClient) Start(ctx context.Context) error {
	var result error
	err := cl.client.CallContext(
		ctx,
		&result,
		"admin_start")
	if err != nil {
		return fmt.Errorf("failed to start Supervisor: %w", err)
	}
	return result
}

func (cl *SupervisorClient) AddL2RPC(ctx context.Context, rpc string) error {
	var result error
	err := cl.client.CallContext(
		ctx,
		&result,
		"admin_addL2RPC",
		rpc)
	if err != nil {
		return fmt.Errorf("failed to Add L2 to Supervisor (rpc: %s): %w", rpc, err)
	}
	return result
}

func (cl *SupervisorClient) CheckMessage(ctx context.Context, identifier types.Identifier, logHash common.Hash) (types.SafetyLevel, error) {
	var result types.SafetyLevel
	err := cl.client.CallContext(
		ctx,
		&result,
		"supervisor_checkMessage",
		identifier,
		logHash)
	if err != nil {
		return types.Invalid, fmt.Errorf("failed to check message (chain %s), (block %v), (index %v), (logHash %s): %w",
			identifier.ChainID,
			identifier.BlockNumber,
			identifier.LogIndex,
			logHash,
			err)
	}
	return result, nil
}

func (cl *SupervisorClient) UnsafeView(ctx context.Context, chainID types.ChainID, unsafe types.ReferenceView) (types.ReferenceView, error) {
	var result types.ReferenceView
	err := cl.client.CallContext(
		ctx,
		&result,
		"supervisor_unsafeView",
		chainID,
		unsafe)
	if err != nil {
		return types.ReferenceView{}, fmt.Errorf("failed to share unsafe block view %s (chain %s): %w", unsafe, chainID, err)
	}
	return result, nil
}

func (cl *SupervisorClient) SafeView(ctx context.Context, chainID types.ChainID, safe types.ReferenceView) (types.ReferenceView, error) {
	var result types.ReferenceView
	err := cl.client.CallContext(
		ctx,
		&result,
		"supervisor_safeView",
		chainID,
		safe)
	if err != nil {
		return types.ReferenceView{}, fmt.Errorf("failed to share safe block view %s (chain %s): %w", safe, chainID, err)
	}
	return result, nil
}

func (cl *SupervisorClient) Finalized(ctx context.Context, chainID types.ChainID) (eth.BlockID, error) {
	var result eth.BlockID
	err := cl.client.CallContext(
		ctx,
		&result,
		"supervisor_finalized",
		chainID)
	return result, err
}

func (cl *SupervisorClient) CrossDerivedFrom(ctx context.Context, chainID types.ChainID, derived eth.BlockID) (eth.BlockRef, error) {
	var result eth.BlockRef
	err := cl.client.CallContext(
		ctx,
		&result,
		"supervisor_crossDerivedFrom",
		chainID,
		derived)
	return result, err
}

func (cl *SupervisorClient) UpdateLocalUnsafe(ctx context.Context, chainID types.ChainID, head eth.BlockRef) error {
	return cl.client.CallContext(
		ctx,
		nil,
		"supervisor_updateLocalUnsafe",
		chainID,
		head)
}

func (cl *SupervisorClient) UpdateLocalSafe(ctx context.Context, chainID types.ChainID, derivedFrom eth.L1BlockRef, lastDerived eth.BlockRef) error {
	return cl.client.CallContext(
		ctx,
		nil,
		"supervisor_updateLocalSafe",
		chainID,
		derivedFrom,
		lastDerived)
}

func (cl *SupervisorClient) UpdateFinalizedL1(ctx context.Context, chainID types.ChainID, finalizedL1 eth.L1BlockRef) error {
	return cl.client.CallContext(
		ctx,
		nil,
		"supervisor_updateFinalizedL1",
		chainID,
		finalizedL1)
}

func (cl *SupervisorClient) Close() {
	cl.client.Close()
}
