package l1

import (
	"context"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
)

var (
	ErrNotFound     = ethereum.NotFound
	ErrUnknownLabel = errors.New("unknown label")
)

type OracleL1Client struct {
	logger               log.Logger
	oracle               Oracle
	head                 eth.L1BlockRef
	hashByNum            map[uint64]common.Hash
	earliestIndexedBlock eth.L1BlockRef
}

func NewOracleL1Client(logger log.Logger, oracle Oracle, l1Head common.Hash) *OracleL1Client {
	head := eth.InfoToL1BlockRef(oracle.HeaderByBlockHash(l1Head))
	logger.Info("L1 head loaded", "hash", head.Hash, "number", head.Number)
	return &OracleL1Client{
		logger:               logger,
		oracle:               oracle,
		head:                 head,
		hashByNum:            map[uint64]common.Hash{head.Number: head.Hash},
		earliestIndexedBlock: head,
	}
}

func (o *OracleL1Client) L1BlockRefByLabel(ctx context.Context, label eth.BlockLabel) (eth.L1BlockRef, error) {
	if label != eth.Unsafe && label != eth.Safe && label != eth.Finalized {
		return eth.L1BlockRef{}, fmt.Errorf("%w: %s", ErrUnknownLabel, label)
	}
	// The L1 head is pre-agreed and unchanging so it can be used for all of unsafe, safe and finalized
	return o.head, nil
}

func (o *OracleL1Client) L1BlockRefByNumber(ctx context.Context, number uint64) (eth.L1BlockRef, error) {
	if number > o.head.Number {
		return eth.L1BlockRef{}, fmt.Errorf("%w: block number %d", ErrNotFound, number)
	}
	hash, ok := o.hashByNum[number]
	if ok {
		return o.L1BlockRefByHash(ctx, hash)
	}
	block := o.earliestIndexedBlock
	o.logger.Info("Extending block by number lookup", "from", block.Number, "to", number)
	for block.Number > number {
		block = eth.InfoToL1BlockRef(o.oracle.HeaderByBlockHash(block.ParentHash))
		o.hashByNum[block.Number] = block.Hash
		o.earliestIndexedBlock = block
	}
	return block, nil
}

func (o *OracleL1Client) L1BlockRefByHash(ctx context.Context, hash common.Hash) (eth.L1BlockRef, error) {
	return eth.InfoToL1BlockRef(o.oracle.HeaderByBlockHash(hash)), nil
}

func (o *OracleL1Client) InfoByHash(ctx context.Context, hash common.Hash) (eth.BlockInfo, error) {
	return o.oracle.HeaderByBlockHash(hash), nil
}

func (o *OracleL1Client) FetchReceipts(ctx context.Context, blockHash common.Hash) (eth.BlockInfo, types.Receipts, error) {
	info, rcpts := o.oracle.ReceiptsByBlockHash(blockHash)
	return info, rcpts, nil
}

func (o *OracleL1Client) InfoAndTxsByHash(ctx context.Context, hash common.Hash) (eth.BlockInfo, []eth.GenericTx, error) {
	info, txs := o.oracle.TransactionsByBlockHash(hash)
	return info, txs, nil
}
