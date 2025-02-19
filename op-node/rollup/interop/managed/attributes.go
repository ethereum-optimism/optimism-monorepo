package managed

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/misc/eip1559"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum-optimism/optimism/op-service/eth"
)

var (
	OptimisticBlockDepositSenderAddress = common.HexToAddress("0xdeaddeaddeaddeaddeaddeaddeaddeaddead0002")

	ErrNotReplacementBlock = errors.New("not a replacement block")
)

// AttributesToReplaceInvalidBlock builds the payload-attributes to replace an invalidated block.
// See https://github.com/ethereum-optimism/specs/blob/main/specs/interop/derivation.md#replacing-invalid-blocks
func AttributesToReplaceInvalidBlock(invalidatedBlock *eth.ExecutionPayloadEnvelope) *eth.PayloadAttributes {
	var replaceTxs []eth.Data

	// Collect all deposit transactions
	for _, tx := range invalidatedBlock.ExecutionPayload.Transactions {
		if len(tx) > 0 && tx[0] == types.DepositTxType {
			replaceTxs = append(replaceTxs, tx)
		}
	}
	// Add the system-tx that declares the replacement.
	if invalidatedBlock.ExecutionPayload.WithdrawalsRoot == nil {
		panic("withdrawals-root is nil")
	}
	l2Output := eth.OutputV0{
		StateRoot:                invalidatedBlock.ExecutionPayload.StateRoot,
		MessagePasserStorageRoot: eth.Bytes32(*invalidatedBlock.ExecutionPayload.WithdrawalsRoot),
		BlockHash:                invalidatedBlock.ExecutionPayload.BlockHash,
	}
	outputRootPreimage := l2Output.Marshal()
	invalidatedBlockTx := InvalidatedBlockSourceDepositTx(outputRootPreimage)
	invalidatedBlockOpaqueTx, err := invalidatedBlockTx.MarshalBinary()
	if err != nil {
		panic(fmt.Errorf("failed to encode system-deposit: %w", err))
	}
	replaceTxs = append(replaceTxs, invalidatedBlockOpaqueTx)

	// Optional engine-API attribute, because of L1-compat, and thus we need a pointer to it.
	gasLimit := invalidatedBlock.ExecutionPayload.GasLimit

	// unfortunately, the engine API needs the inner value, not the extra-data.
	// So we translate it here.
	extraData := invalidatedBlock.ExecutionPayload.ExtraData
	denominator, elasticity := eip1559.DecodeHoloceneExtraData(extraData)
	eip1559Params := eth.Bytes8(eip1559.EncodeHolocene1559Params(denominator, elasticity))

	attrs := &eth.PayloadAttributes{
		Timestamp:             invalidatedBlock.ExecutionPayload.Timestamp,
		PrevRandao:            invalidatedBlock.ExecutionPayload.PrevRandao,
		SuggestedFeeRecipient: invalidatedBlock.ExecutionPayload.FeeRecipient,
		Withdrawals:           invalidatedBlock.ExecutionPayload.Withdrawals,
		ParentBeaconBlockRoot: invalidatedBlock.ParentBeaconBlockRoot,
		Transactions:          replaceTxs,
		NoTxPool:              true,
		GasLimit:              &gasLimit,
		EIP1559Params:         &eip1559Params,
	}
	return attrs
}

func InvalidatedBlockSourceDepositTx(outputRootPreimage []byte) *types.Transaction {
	outputRoot := crypto.Keccak256Hash(outputRootPreimage)
	src := derive.InvalidatedBlockSource{OutputRoot: outputRoot}
	return types.NewTx(&types.DepositTx{
		SourceHash:          src.SourceHash(),
		From:                OptimisticBlockDepositSenderAddress,
		To:                  &common.Address{}, // to the zero address, no EVM execution.
		Mint:                big.NewInt(0),
		Value:               big.NewInt(0),
		Gas:                 36_000,
		IsSystemTransaction: false,
		Data:                outputRootPreimage,
	})
}

func DecodeInvalidatedBlockTxFromReplacement(txs []eth.Data) (*eth.OutputV0, error) {
	if len(txs) == 0 {
		return nil, errors.New("expected block-replacement tx and more")
	}
	var tx types.Transaction
	if err := tx.UnmarshalBinary(txs[len(txs)-1]); err != nil {
		return nil, fmt.Errorf("failed to unmarshal invalidated-block system tx: %w", err)
	}
	return DecodeInvalidatedBlockTx(&tx)
}

func DecodeInvalidatedBlockTx(tx *types.Transaction) (*eth.OutputV0, error) {
	if tx.Type() != types.DepositTxType {
		return nil, fmt.Errorf("%w: expected deposit tx type, but got %d", ErrNotReplacementBlock, tx.Type())
	}
	signer := types.LatestSignerForChainID(tx.ChainId())
	from, err := signer.Sender(tx)
	if err != nil {
		return nil, fmt.Errorf("failed to get invalidated-block deposit-tx sender addr: %w", err)
	}
	if from != OptimisticBlockDepositSenderAddress {
		return nil, fmt.Errorf("%w: expected system tx sender, but got %s", ErrNotReplacementBlock, from)
	}
	out, err := eth.UnmarshalOutput(tx.Data())
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal output-root preimage: %w", err)
	}
	outV0, ok := out.(*eth.OutputV0)
	if !ok {
		return nil, fmt.Errorf("expected output v0 preimage, but got %T", out)
	}
	return outV0, nil
}
