package sources

import (
	"encoding/json"
	"math/big"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"

	"github.com/ethereum-optimism/optimism/op-service/testutils"
)

func TestRawJsonTransaction(t *testing.T) {

	key := testutils.RandomKey()
	chainID := big.NewInt(1)
	signer := types.LatestSignerForChainID(chainID)
	rng := rand.New(rand.NewSource(1234))
	to := testutils.RandomAddress(rng)
	txInner := &types.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     uint64(123),
		GasTipCap: big.NewInt(100_000),
		GasFeeCap: big.NewInt(1000_000),
		Gas:       24_000,
		To:        &to,
		Value:     big.NewInt(123456),
		Data:      []byte("hello world"),
		AccessList: types.AccessList{
			types.AccessTuple{
				Address: testutils.RandomAddress(rng),
				StorageKeys: []common.Hash{
					testutils.RandomHash(rng),
				},
			},
		},
	}
	tx, err := types.SignNewTx(key, signer, txInner)
	require.NoError(t, err)
	txJson, err := json.Marshal(tx)
	require.NoError(t, err)
	t.Log(string(txJson))

	// Test json round trip
	var flexTx RawJsonTransaction

	// Takes JSON encoded DynamicFeeTx and converts it to RawJsonTransaction:
	require.NoError(t, json.Unmarshal(txJson, &flexTx))
	// Takes RawJsonTransaction and JSON encodes it (uses cached raw JSON from the unmarshalling step):
	reEncoded, err := json.Marshal(&flexTx)
	require.NoError(t, err)
	require.Equal(t, hexutil.Bytes(txJson), hexutil.Bytes(reEncoded))

	require.Equal(t, tx.Hash(), flexTx.TxHash())
	require.Equal(t, tx.Type(), flexTx.TxType())

	// Test binary round trip
	// Takes RawJsonTransaction and converts it to binary, this requires the tx to be one of the supported types:
	data, err := flexTx.MarshalBinary()
	require.NoError(t, err)
	t.Log(hexutil.Encode(data))
	t.Log(tx.Hash(), flexTx.TxHash())

	// Unmarshal the binary data back into a new RawJsonTransaction, again this requires it to be one of the supported types:
	var reDecoded RawJsonTransaction
	require.NoError(t, reDecoded.UnmarshalBinary(data))

	// Re-encode the RawJsonTransaction as JSON:
	jsonAgain, err := json.Marshal(&reDecoded)
	require.NoError(t, err)
	require.Equal(t, hexutil.Bytes(txJson), hexutil.Bytes(jsonAgain))
}
