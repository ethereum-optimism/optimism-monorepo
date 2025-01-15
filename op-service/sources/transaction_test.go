package sources

import (
	"encoding/json"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"

	"github.com/ethereum-optimism/optimism/op-service/testutils"
)

func TestRawJsonTransaction(t *testing.T) {
	rng := rand.New(rand.NewSource(1234))
	tx := testutils.RandomDynamicFeeTx(rng, testutils.RandomSigner(rng))

	txJson, err := json.Marshal(tx)
	require.NoError(t, err)

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

	// Unmarshal the binary data back into a new RawJsonTransaction, again this requires it to be one of the supported types:
	var reDecoded RawJsonTransaction
	require.NoError(t, reDecoded.UnmarshalBinary(data))

	// Re-encode the RawJsonTransaction as JSON:
	jsonAgain, err := json.Marshal(&reDecoded)
	require.NoError(t, err)
	require.Equal(t, string(txJson), string(jsonAgain))

	// A future, unsupported EIP 2718 tx type, unsupported at the time of writing
	hypotheticalTxJson := []byte(`{"hash":"0x9222cd0ffde5ae945a5aa35a58dcc1c8014385bed272a0a86c8852013803c246","type":"0x66"}`)

	// Test json round trip
	// Takes JSON encoded DynamicFeeTx and converts it to RawJsonTransaction:
	require.NoError(t, json.Unmarshal(hypotheticalTxJson, &flexTx))

	// Takes RawJsonTransaction and JSON encodes it (uses cached raw JSON from the unmarshalling step):

	require.Equal(t, common.HexToHash("0x9222cd0ffde5ae945a5aa35a58dcc1c8014385bed272a0a86c8852013803c246"), flexTx.TxHash())
	require.Equal(t, uint8(0x66), flexTx.TxType())

	reEncoded, err = json.Marshal(&flexTx)
	require.NoError(t, err)
	require.Equal(t, string(hypotheticalTxJson), string(reEncoded))

	// Unsupported tx type should not be able to be converted to binary
	data, err = flexTx.MarshalBinary()
	require.Error(t, err)
	require.Nil(t, data)

}
