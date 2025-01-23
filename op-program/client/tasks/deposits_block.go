package tasks

import (
	"context"
	"fmt"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-program/client/l1"
	"github.com/ethereum-optimism/optimism/op-program/client/l2"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/misc/eip1559"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
)

// BuildDepositOnlyBlock builds a deposits-only block replacement for the specified optimistic block and returns the block hash and output root
// for the new block.
// The specified l2OutputRoot must be the output root of the optimistic block's parent.
func BuildDepositOnlyBlock(
	logger log.Logger,
	cfg *rollup.Config,
	l2Cfg *params.ChainConfig,
	optimisticBlock *types.Block,
	l1Head common.Hash,
	agreedL2OutputRoot eth.Bytes32,
	l1Oracle l1.Oracle,
	l2Oracle l2.Oracle,
) (common.Hash, eth.Bytes32, error) {
	engineBackend, err := l2.NewOracleBackedL2Chain(logger, l2Oracle, l1Oracle /* kzg oracle */, l2Cfg, common.Hash(agreedL2OutputRoot))
	if err != nil {
		return common.Hash{}, eth.Bytes32{}, fmt.Errorf("failed to create oracle-backed L2 chain: %w", err)
	}
	l2Source := l2.NewOracleEngine(cfg, logger, engineBackend)
	l2Head := l2Oracle.BlockByHash(optimisticBlock.ParentHash(), l2Cfg.ChainID.Uint64())
	l2HeadHash := l2Head.Hash()

	logger.Info("Building a deposts-only block to replace block %v", optimisticBlock.Hash())
	attrs, err := blockToDepositsOnlyAttributes(cfg, optimisticBlock)
	if err != nil {
		return common.Hash{}, eth.Bytes32{}, fmt.Errorf("failed to convert block to deposits-only attributes: %w", err)
	}
	result, err := l2Source.ForkchoiceUpdate(context.Background(), &eth.ForkchoiceState{
		HeadBlockHash:      l2HeadHash,
		SafeBlockHash:      l2HeadHash,
		FinalizedBlockHash: l2HeadHash,
	}, attrs)
	if err != nil {
		return common.Hash{}, eth.Bytes32{}, fmt.Errorf("failed to update forkchoice state: %w", err)
	}
	if result.PayloadStatus.Status != eth.ExecutionValid {
		return common.Hash{}, eth.Bytes32{}, fmt.Errorf("failed to update forkchoice state: %w", eth.ForkchoiceUpdateErr(result.PayloadStatus))
	}

	id := result.PayloadID
	if id == nil {
		return common.Hash{}, eth.Bytes32{}, fmt.Errorf("nil id in forkchoice result when expecting a valid ID")
	}
	payload, err := l2Source.GetPayload(context.Background(), eth.PayloadInfo{ID: *id, Timestamp: uint64(attrs.Timestamp)})
	if err != nil {
		return common.Hash{}, eth.Bytes32{}, fmt.Errorf("failed to get payload: %w", err)
	}
	blockHash, outputRoot, err := l2Source.L2OutputRoot(uint64(payload.ExecutionPayload.BlockNumber))
	if err != nil {
		return common.Hash{}, eth.Bytes32{}, fmt.Errorf("failed to get L2 output root: %w", err)
	}
	return blockHash, outputRoot, nil
}

func blockToDepositsOnlyAttributes(cfg *rollup.Config, block *types.Block) (*eth.PayloadAttributes, error) {
	gasLimit := eth.Uint64Quantity(block.GasLimit())
	withdrawals := block.Withdrawals()
	var deposits []eth.Data
	for _, tx := range block.Transactions() {
		if tx.Type() == types.DepositTxType {
			txdata, err := tx.MarshalBinary()
			if err != nil {
				return nil, err
			}
			deposits = append(deposits, txdata)
		}
	}
	attrs := &eth.PayloadAttributes{
		Timestamp:             eth.Uint64Quantity(block.Time()),
		PrevRandao:            eth.Bytes32(block.MixDigest()),
		SuggestedFeeRecipient: block.Coinbase(),
		Withdrawals:           &withdrawals,
		ParentBeaconBlockRoot: block.BeaconRoot(),
		Transactions:          deposits,
		NoTxPool:              true,
		GasLimit:              &gasLimit,
	}
	if cfg.IsHolocene(block.Time()) {
		d, e := eip1559.DecodeHoloceneExtraData(block.Extra())
		eip1559Params := eip1559.EncodeHolocene1559Params(d, e)
		copy(attrs.EIP1559Params[:], eip1559Params)
	}
	return attrs, nil
}
