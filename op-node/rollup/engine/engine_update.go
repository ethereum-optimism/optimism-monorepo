package engine

import (
	"context"
	"errors"
	"fmt"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
)

// isDepositTx checks an opaqueTx to determine if it is a Deposit Transaction
// It has to return an error in the case the transaction is empty
func isDepositTx(opaqueTx eth.Data) (bool, error) {
	if len(opaqueTx) == 0 {
		return false, errors.New("empty transaction")
	}
	return opaqueTx[0] == types.DepositTxType, nil
}

// lastDeposit finds the index of last deposit at the start of the transactions.
// It walks the transactions from the start until it finds a non-deposit tx.
// An error is returned if any looked at transaction cannot be decoded
func lastDeposit(txns []eth.Data) (int, error) {
	var lastDeposit int
	for i, tx := range txns {
		deposit, err := isDepositTx(tx)
		if err != nil {
			return 0, fmt.Errorf("invalid transaction at idx %d", i)
		}
		if deposit {
			lastDeposit = i
		} else {
			break
		}
	}
	return lastDeposit, nil
}

func sanityCheckPayload(payload *eth.ExecutionPayload) error {
	// Sanity check payload before inserting it
	if len(payload.Transactions) == 0 {
		return errors.New("no transactions in returned payload")
	}
	if payload.Transactions[0][0] != types.DepositTxType {
		return fmt.Errorf("first transaction was not deposit tx. Got %v", payload.Transactions[0][0])
	}
	// Ensure that the deposits are first
	lastDeposit, err := lastDeposit(payload.Transactions)
	if err != nil {
		return fmt.Errorf("failed to find last deposit: %w", err)
	}
	// Ensure no deposits after last deposit
	for i := lastDeposit + 1; i < len(payload.Transactions); i++ {
		tx := payload.Transactions[i]
		deposit, err := isDepositTx(tx)
		if err != nil {
			return fmt.Errorf("failed to decode transaction idx %d: %w", i, err)
		}
		if deposit {
			return fmt.Errorf("deposit tx (%d) after other tx in l2 block with prev deposit at idx %d", i, lastDeposit)
		}
	}
	return nil
}

var ErrEngineSyncing = errors.New("engine is syncing")

type BlockInsertionErrType uint

const (
	// BlockInsertOK indicates that the payload was successfully executed and appended to the canonical chain.
	BlockInsertOK BlockInsertionErrType = iota
	// BlockInsertTemporaryErr indicates that the insertion failed but may succeed at a later time without changes to the payload.
	BlockInsertTemporaryErr
	// BlockInsertPrestateErr indicates that the pre-state to insert the payload could not be prepared, e.g. due to missing chain data.
	BlockInsertPrestateErr
	// BlockInsertPayloadErr indicates that the payload was invalid and cannot become canonical.
	BlockInsertPayloadErr
)

// startPayload starts an execution payload building process in the provided Engine, with the given attributes.
// The severity of the error is distinguished to determine whether the same payload attributes may be re-attempted later.
func startPayload(ctx context.Context, eng ExecEngine, fc eth.ForkchoiceState, attrs *eth.PayloadAttributes) (id eth.PayloadID, errType BlockInsertionErrType, err error) {
	fcRes, err := eng.ForkchoiceUpdate(ctx, &fc, attrs)
	if err != nil {
		var rpcErr rpc.Error
		if errors.As(err, &rpcErr) {
			switch code := eth.ErrorCode(rpcErr.ErrorCode()); code {
			case eth.InvalidForkchoiceState:
				return eth.PayloadID{}, BlockInsertPrestateErr, fmt.Errorf("pre-block-creation forkchoice update was inconsistent with engine, need reset to resolve: %w", rpcErr)
			case eth.InvalidPayloadAttributes:
				return eth.PayloadID{}, BlockInsertPayloadErr, fmt.Errorf("payload attributes are not valid, cannot build block: %w", rpcErr)
			default:
				if code.IsEngineError() {
					return eth.PayloadID{}, BlockInsertPrestateErr, fmt.Errorf("unexpected engine error code in forkchoice-updated response: %w", err)
				} else {
					return eth.PayloadID{}, BlockInsertTemporaryErr, fmt.Errorf("unexpected generic error code in forkchoice-updated response: %w", err)
				}
			}
		} else {
			return eth.PayloadID{}, BlockInsertTemporaryErr, fmt.Errorf("failed to create new block via forkchoice: %w", err)
		}
	}

	switch fcRes.PayloadStatus.Status {
	// TODO: snap sync - specify explicit different error type if node is syncing
	case eth.ExecutionInvalid, eth.ExecutionInvalidBlockHash:
		return eth.PayloadID{}, BlockInsertPayloadErr, eth.ForkchoiceUpdateErr(fcRes.PayloadStatus)
	case eth.ExecutionValid:
		id := fcRes.PayloadID
		if id == nil {
			return eth.PayloadID{}, BlockInsertTemporaryErr, errors.New("nil id in forkchoice result when expecting a valid ID")
		}
		return *id, BlockInsertOK, nil
	case eth.ExecutionSyncing:
		return eth.PayloadID{}, BlockInsertTemporaryErr, ErrEngineSyncing
	default:
		return eth.PayloadID{}, BlockInsertTemporaryErr, eth.ForkchoiceUpdateErr(fcRes.PayloadStatus)
	}
}
