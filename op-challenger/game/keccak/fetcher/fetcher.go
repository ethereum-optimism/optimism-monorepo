package fetcher

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/contracts"
	keccakTypes "github.com/ethereum-optimism/optimism/op-challenger/game/keccak/types"
	"github.com/ethereum-optimism/optimism/op-service/sources/batching/rpcblock"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
)

var (
	ErrNoLeavesFound = errors.New("no leaves found in block")
)

type L1Source interface {
	BlockByNumber(ctx context.Context, number *big.Int) (*types.Block, error)
	TransactionReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error)
	ChainID(ctx context.Context) (*big.Int, error)
}

type Oracle interface {
	Addr() common.Address
	GetInputDataBlocks(ctx context.Context, block rpcblock.Block, ident keccakTypes.LargePreimageIdent) ([]uint64, error)
	DecodeInputData(data []byte) (*big.Int, keccakTypes.InputData, error)
}

type InputFetcher struct {
	log    log.Logger
	source L1Source
}

func (f *InputFetcher) FetchInputs(ctx context.Context, blockHash common.Hash, oracle Oracle, ident keccakTypes.LargePreimageIdent) ([]keccakTypes.InputData, error) {
	blockNums, err := oracle.GetInputDataBlocks(ctx, rpcblock.ByHash(blockHash), ident)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve leaf block nums: %w", err)
	}
	var inputs []keccakTypes.InputData
	for _, blockNum := range blockNums {
		foundRelevantTx := false
		block, err := f.source.BlockByNumber(ctx, new(big.Int).SetUint64(blockNum))
		if err != nil {
			return nil, fmt.Errorf("failed getting tx for block %v: %w", blockNum, err)
		}
		for _, tx := range block.Transactions() {
			inputData, err := f.extractRelevantLeavesFromTx(ctx, oracle, tx, ident)
			if err != nil {
				return nil, err
			}
			if len(inputData) > 0 {
				foundRelevantTx = true
				inputs = append(inputs, inputData...)
			}
		}
		if !foundRelevantTx {
			// The contract said there was a relevant transaction in this block that we failed to find.
			// There was either a reorg or the extraction logic is broken.
			// Either way, abort this attempt to validate the preimage.
			return nil, fmt.Errorf("%w %v", ErrNoLeavesFound, blockNum)
		}
	}
	return inputs, nil
}

func (f *InputFetcher) extractRelevantLeavesFromTx(ctx context.Context, oracle Oracle, tx *types.Transaction, ident keccakTypes.LargePreimageIdent) ([]keccakTypes.InputData, error) {
	rcpt, err := f.source.TransactionReceipt(ctx, tx.Hash())
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve receipt for tx %v: %w", tx.Hash(), err)
	}
	if rcpt.Status != types.ReceiptStatusSuccessful {
		f.log.Trace("Skipping transaction with failed receipt status", "tx", tx.Hash(), "status", rcpt.Status)
		return nil, nil
	}

	// Iterate over the logs from in this receipt, looking for relevant logs emitted from the oracle contract
	var inputs []keccakTypes.InputData
	for i, txLog := range rcpt.Logs {
		if txLog.Address != oracle.Addr() {
			f.log.Trace("Skip tx log not emitted by the oracle contract", "tx", tx.Hash(), "logIndex", i, "targetContract", oracle.Addr(), "actualContract", txLog.Address)
			continue
		}
		if len(txLog.Data) < 20 {
			f.log.Trace("Skip tx log with insufficient data (less than 20 bytes)", "tx", tx.Hash(), "logIndex", i, "dataLength", len(txLog.Data))
			continue
		}
		caller := common.Address(txLog.Data[0:20])
		callData := txLog.Data[20:]

		if caller != ident.Claimant {
			f.log.Trace("Skip tx log from irrelevant claimant", "tx", tx.Hash(), "logIndex", i, "targetClaimant", ident.Claimant, "actualClaimant", caller)
			continue
		}
		uuid, inputData, err := oracle.DecodeInputData(callData)
		if errors.Is(err, contracts.ErrInvalidAddLeavesCall) {
			f.log.Trace("Skip tx log with call data not targeting expected method", "tx", tx.Hash(), "logIndex", i, "err", err)
			continue
		} else if err != nil {
			return nil, err
		}
		if uuid.Cmp(ident.UUID) != 0 {
			f.log.Trace("Skip tx log with irrelevant UUID", "tx", tx.Hash(), "logIndex", i, "targetUUID", ident.UUID, "actualUUID", uuid)
			continue
		}
		inputs = append(inputs, inputData)
	}

	return inputs, nil
}

func NewPreimageFetcher(logger log.Logger, source L1Source) *InputFetcher {
	return &InputFetcher{
		log:    logger,
		source: source,
	}
}
