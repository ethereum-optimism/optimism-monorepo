package tasks

import (
	"context"
	"encoding/binary"
	"fmt"
	"math/big"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum-optimism/optimism/op-program/client/l1"
	"github.com/ethereum-optimism/optimism/op-program/client/l2"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/misc/eip1559"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
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
	optimisticBlockOutput *eth.OutputV0,
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
	attrs, err := blockToDepositsOnlyAttributes(cfg, optimisticBlock, optimisticBlockOutput)
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

func blockToDepositsOnlyAttributes(cfg *rollup.Config, block *types.Block, output *eth.OutputV0) (*eth.PayloadAttributes, error) {
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
	depositedTx := createBlockDepositedTx(output)
	depositedTxData, err := depositedTx.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal deposited tx: %w", err)
	}
	deposits = append(deposits, depositedTxData)

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

func createBlockDepositedTx(output *eth.OutputV0) *types.Transaction {
	// TODO: refactor this with block replacement helpers introduced in https://github.com/ethereum-optimism/optimism/pull/13645
	outputRoot := eth.OutputRoot(output)
	outputRootPreimage := output.Marshal()

	sourceHash := createBlockDepositedSourceHash(outputRoot)
	return types.NewTx(&types.DepositTx{
		SourceHash:          sourceHash,
		From:                derive.L1InfoDepositerAddress,
		To:                  &common.Address{}, // to the zero address, no EVM execution.
		Mint:                big.NewInt(0),
		Value:               big.NewInt(0),
		Gas:                 36_000,
		IsSystemTransaction: false,
		Data:                outputRootPreimage,
	})
}

func createBlockDepositedSourceHash(outputRoot eth.Bytes32) common.Hash {
	domain := uint64(4)
	var domainInput [32 * 2]byte
	binary.BigEndian.PutUint64(domainInput[32-8:32], domain)
	copy(domainInput[32:], outputRoot[:])
	return crypto.Keccak256Hash(domainInput[:])
}
