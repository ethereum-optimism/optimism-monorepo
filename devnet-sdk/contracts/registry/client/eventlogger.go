package client

import (
	"context"
	"fmt"

	"github.com/ethereum-optimism/optimism/devnet-sdk/contracts/bindings"
	"github.com/ethereum-optimism/optimism/devnet-sdk/interfaces"
	"github.com/ethereum-optimism/optimism/devnet-sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

type eventLoggerBinding struct {
	contractAddress types.Address
	client          *ethclient.Client
	binding         *bindings.EventLogger
}

var _ interfaces.EventLogger = (*eventLoggerBinding)(nil)

func (b *eventLoggerBinding) EmitLog(topics []types.Topic, data []byte) types.WriteInvocation[any] {
	return &eventLoggerEmitLogImpl{
		contract: b,
		topics:   topics,
		data:     data,
	}
}

func (b *eventLoggerBinding) ValidateMessage(id types.Identifier, msgHash types.Hash) types.WriteInvocation[any] {
	return &eventLoggerValidateMessageImpl{
		contract: b,
		id:       id,
		msgHash:  msgHash,
	}
}

type eventLoggerEmitLogImpl struct {
	contract *eventLoggerBinding
	topics   []types.Topic
	data     []byte
}

func (i *eventLoggerEmitLogImpl) Call(ctx context.Context) (any, error) {
	opts := &bind.TransactOpts{
		From: i.contract.contractAddress,
		Signer: func(address common.Address, tx *gethTypes.Transaction) (*gethTypes.Transaction, error) {
			return tx, nil
		},
	}

	// Convert topics to [][32]byte
	rawTopics := make([][32]byte, len(i.topics))
	for j, topic := range i.topics {
		rawTopics[j] = [32]byte(topic)
	}

	tx, err := i.contract.binding.EmitLog(opts, rawTopics, i.data)
	if err != nil {
		return nil, fmt.Errorf("failed to emit log: %w", err)
	}

	return tx, nil
}

func (i *eventLoggerEmitLogImpl) Send(ctx context.Context) types.InvocationResult {
	tx, err := i.Call(ctx)
	return &eventLoggerEmitLogResult{
		contract: i.contract,
		tx:       tx,
		err:      err,
	}
}

type eventLoggerValidateMessageImpl struct {
	contract *eventLoggerBinding
	id       types.Identifier
	msgHash  types.Hash
}

func (i *eventLoggerValidateMessageImpl) Call(ctx context.Context) (any, error) {
	opts := &bind.TransactOpts{
		From: i.contract.contractAddress,
		Signer: func(address common.Address, tx *gethTypes.Transaction) (*gethTypes.Transaction, error) {
			return tx, nil
		},
	}

	bindingsId := i.id

	tx, err := i.contract.binding.ValidateMessage(opts, bindingsId, i.msgHash)
	if err != nil {
		return nil, fmt.Errorf("failed to validate message: %w", err)
	}

	return tx, nil
}

func (i *eventLoggerValidateMessageImpl) Send(ctx context.Context) types.InvocationResult {
	tx, err := i.Call(ctx)
	return &eventLoggerEmitLogResult{
		contract: i.contract,
		tx:       tx,
		err:      err,
	}
}

type eventLoggerEmitLogResult struct {
	contract *eventLoggerBinding
	tx       any
	err      error
}

func (r *eventLoggerEmitLogResult) Error() error {
	return r.err
}

func (r *eventLoggerEmitLogResult) Wait() error {
	if r.err != nil {
		return r.err
	}
	if r.tx == nil {
		return fmt.Errorf("no transaction to wait for")
	}

	if tx, ok := r.tx.(*gethTypes.Transaction); ok {
		receipt, err := bind.WaitMined(context.Background(), r.contract.client, tx)
		if err != nil {
			return fmt.Errorf("failed waiting for transaction confirmation: %w", err)
		}

		if receipt.Status == 0 {
			return fmt.Errorf("transaction failed")
		}
	}

	return nil
}
