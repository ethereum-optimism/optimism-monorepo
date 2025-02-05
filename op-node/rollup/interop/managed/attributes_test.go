package managed

import (
	"math/big"
	"math/rand" // nosemgrep
	"testing"

	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/consensus/misc/eip1559"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testutils"
)

// TestAttributesToReplaceInvalidBlock creates a block to invalidate,
// and then constructs the block-attributes to build the replacement block with,
// and tests that these accurately reference the invalidated block in the optimistic block tx.
func TestAttributesToReplaceInvalidBlock(t *testing.T) {
	rng := rand.New(rand.NewSource(1234))
	signer := types.LatestSignerForChainID(big.NewInt(123))
	tx := testutils.RandomDynamicFeeTx(rng, signer)

	exampleDeposit := types.NewTx(&types.DepositTx{
		SourceHash:          testutils.RandomHash(rng),
		From:                testutils.RandomAddress(rng),
		To:                  nil,
		Mint:                big.NewInt(0),
		Value:               big.NewInt(0),
		Gas:                 30_000,
		IsSystemTransaction: false,
		Data:                []byte("hello"),
	})
	opaqueDepositTx, err := exampleDeposit.MarshalBinary()
	require.NoError(t, err)

	opaqueUserTx, err := tx.MarshalBinary()
	require.NoError(t, err)

	denominator := uint64(100)
	elasticity := uint64(42)
	extraData := eip1559.EncodeHoloceneExtraData(denominator, elasticity)
	withdrawalsRoot := testutils.RandomHash(rng)

	beaconRoot := testutils.RandomHash(rng)
	invalidatedBlock := &eth.ExecutionPayloadEnvelope{
		ParentBeaconBlockRoot: &beaconRoot,
		ExecutionPayload: &eth.ExecutionPayload{
			ParentHash:    testutils.RandomHash(rng),
			FeeRecipient:  common.Address{},
			StateRoot:     eth.Bytes32(testutils.RandomHash(rng)),
			ReceiptsRoot:  eth.Bytes32(testutils.RandomHash(rng)),
			LogsBloom:     eth.Bytes256{},
			PrevRandao:    eth.Bytes32(testutils.RandomHash(rng)),
			BlockNumber:   eth.Uint64Quantity(rng.Uint64()),
			GasLimit:      eth.Uint64Quantity(30_000_000),
			GasUsed:       eth.Uint64Quantity(1_000_000),
			Timestamp:     eth.Uint64Quantity(rng.Uint64()),
			ExtraData:     extraData,
			BaseFeePerGas: eth.Uint256Quantity(*uint256.NewInt(7)),
			BlockHash:     testutils.RandomHash(rng),
			Transactions: []eth.Data{
				opaqueDepositTx,
				opaqueUserTx,
			},
			Withdrawals:     &types.Withdrawals{},
			BlobGasUsed:     new(eth.Uint64Quantity),
			ExcessBlobGas:   new(eth.Uint64Quantity),
			WithdrawalsRoot: &withdrawalsRoot,
		},
	}
	attrs := AttributesToReplaceInvalidBlock(invalidatedBlock)
	require.Equal(t, invalidatedBlock.ExecutionPayload.Timestamp, attrs.Timestamp)
	require.Equal(t, invalidatedBlock.ExecutionPayload.PrevRandao, attrs.PrevRandao)
	require.Equal(t, invalidatedBlock.ExecutionPayload.FeeRecipient, attrs.SuggestedFeeRecipient)
	require.Equal(t, invalidatedBlock.ExecutionPayload.Withdrawals, attrs.Withdrawals)
	require.Equal(t, invalidatedBlock.ParentBeaconBlockRoot, attrs.ParentBeaconBlockRoot)
	require.Len(t, attrs.Transactions, 2)
	require.Equal(t, hexutil.Bytes(opaqueDepositTx).String(), attrs.Transactions[0].String())
	require.Equal(t, uint8(types.DepositTxType), attrs.Transactions[1][0], "remove user tx, add optimistic block system tx")
	require.True(t, attrs.NoTxPool)
	require.Equal(t, invalidatedBlock.ExecutionPayload.GasLimit, *attrs.GasLimit)
	d, e := eip1559.DecodeHolocene1559Params(attrs.EIP1559Params[:])
	require.Equal(t, denominator, d)
	require.Equal(t, elasticity, e)
	result, err := DecodeInvalidatedBlockTxFromReplacement(attrs.Transactions)
	require.NoError(t, err)
	require.Equal(t, invalidatedBlock.ExecutionPayload.BlockHash, result.BlockHash)
	require.Equal(t, invalidatedBlock.ExecutionPayload.StateRoot, result.StateRoot)
	require.Equal(t, withdrawalsRoot[:], result.MessagePasserStorageRoot[:])
}

// TestInvalidatedBlockTx tests we can encode/decode the system tx that represents the invalidated block
func TestInvalidatedBlockTx(t *testing.T) {
	t.Run("nil list", func(t *testing.T) {
		_, err := DecodeInvalidatedBlockTxFromReplacement(nil)
		require.NotNil(t, err)
	})
	t.Run("empty list", func(t *testing.T) {
		_, err := DecodeInvalidatedBlockTxFromReplacement([]eth.Data{})
		require.NotNil(t, err)
	})
	t.Run("success", func(t *testing.T) {
		rng := rand.New(rand.NewSource(1234))
		output := &eth.OutputV0{
			StateRoot:                eth.Bytes32(testutils.RandomHash(rng)),
			MessagePasserStorageRoot: eth.Bytes32(testutils.RandomHash(rng)),
			BlockHash:                testutils.RandomHash(rng),
		}
		outputRootPreimage := output.Marshal()
		outputRoot := crypto.Keccak256Hash(outputRootPreimage)
		tx := InvalidatedBlockSourceDepositTx(outputRootPreimage)
		signer := types.LatestSignerForChainID(big.NewInt(0))
		sender, err := signer.Sender(tx)
		require.NoError(t, err)
		require.Equal(t, OptimisticBlockDepositSenderAddress, sender, "from")
		require.Equal(t, common.Address{}, *tx.To(), "to")
		require.Equal(t, "0", tx.Mint().String(), "mint")
		require.Equal(t, "0", tx.Value().String(), "value")
		require.Equal(t, uint64(36_000), tx.Gas(), "gasLimit")
		require.False(t, tx.IsSystemTx(), "legacy isSystemTx")
		require.Equal(t, outputRootPreimage, tx.Data(), "data")
		domain := uint256.NewInt(4).Bytes32()
		sourceHash := crypto.Keccak256Hash(domain[:], outputRoot[:])
		require.Equal(t, sourceHash, tx.SourceHash(), "sourceHash")
		encoded, err := tx.MarshalBinary()
		require.NoError(t, err, "must encode")
		result, err := DecodeInvalidatedBlockTxFromReplacement([]eth.Data{encoded})
		require.NoError(t, err, "must decode")
		require.Equal(t, *output, *result, "roundtrip success")
	})
	t.Run("other tx type", func(t *testing.T) {
		rng := rand.New(rand.NewSource(1234))
		signer := types.LatestSignerForChainID(big.NewInt(123))
		tx := testutils.RandomDynamicFeeTx(rng, signer)
		encoded, err := tx.MarshalBinary()
		require.NoError(t, err, "must encode")
		_, err = DecodeInvalidatedBlockTxFromReplacement([]eth.Data{encoded})
		require.Error(t, err, "expected deposit")
	})
	t.Run("bad tx sender", func(t *testing.T) {
		rng := rand.New(rand.NewSource(1234))
		tx := types.NewTx(&types.DepositTx{
			SourceHash:          common.Hash{},
			From:                testutils.RandomAddress(rng),
			To:                  &common.Address{},
			Mint:                big.NewInt(0),
			Value:               big.NewInt(0),
			Gas:                 36_000,
			IsSystemTransaction: false,
			Data:                []byte{},
		})
		encoded, err := tx.MarshalBinary()
		require.NoError(t, err, "must encode")
		_, err = DecodeInvalidatedBlockTxFromReplacement([]eth.Data{encoded})
		require.Error(t, err, "expected system tx sender")
	})
	t.Run("bad preimage", func(t *testing.T) {
		tx := InvalidatedBlockSourceDepositTx([]byte("invalid output root preimage"))
		encoded, err := tx.MarshalBinary()
		require.NoError(t, err, "must encode")
		_, err = DecodeInvalidatedBlockTxFromReplacement([]eth.Data{encoded})
		require.Error(t, err, "failed to unmarshal")
	})
}
