package sources

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"

	"github.com/ethereum-optimism/optimism/op-service/testutils"
)

func TestRawJsonTransaction(t *testing.T) {
	rng := rand.New(rand.NewSource(1234))

	getJsonAndHash := func(tx *types.Transaction) ([]byte, common.Hash) {
		txJson, err := tx.MarshalJSON()
		require.NoError(t, err)
		return txJson, tx.Hash()
	}

	tx := testutils.RandomLegacyTx(rng, testutils.RandomSigner(rng))
	legacyTxJSON, legacyTxHash := getJsonAndHash(tx)

	tx = testutils.RandomAccessListTx(rng, testutils.RandomSigner(rng))
	accessListTxJSON, accessListTxHash := getJsonAndHash(tx)

	tx = testutils.RandomDynamicFeeTx(rng, testutils.RandomSigner(rng))
	dynamicFeeTxJSON, dyamicFeeTxHash := getJsonAndHash(tx)

	futureTxJSON := []byte(`{"hash":"0x9222cd0ffde5ae945a5aa35a58dcc1c8014385bed272a0a86c8852013803c246","type":"0x66"}`)
	futureTxHash := common.HexToHash("0x9222cd0ffde5ae945a5aa35a58dcc1c8014385bed272a0a86c8852013803c246")

	nonEIP2718TxJSON := []byte(`{"hash":"0x9222cd0ffde5ae945a5aa35a58dcc1c8014385bed272a0a86c8852013803c246","type":"0x80"}`)
	nonEIP2718TxHash := common.HexToHash("0x9222cd0ffde5ae945a5aa35a58dcc1c8014385bed272a0a86c8852013803c246")

	type testCase struct {
		name      string
		jsonTx    []byte
		txType    uint8
		txHash    common.Hash
		eip2718   bool // is this an EIP 2718 transaction?
		supported bool // is this an explicitly supported EIP 2718 transaction?
	}

	testCases := []testCase{
		{"LegacyTx(0x00)", legacyTxJSON, 0, legacyTxHash, true, true},
		{"AccessListTx(0x01)", accessListTxJSON, 1, accessListTxHash, true, true},
		{"DyamicFeeTx(0x02)", dynamicFeeTxJSON, 2, dyamicFeeTxHash, true, true},
		{"FutureTx(0x66)", futureTxJSON, 102, futureTxHash, true, false},
		{"NonEIP2718Tx(0x80)", nonEIP2718TxJSON, 128, nonEIP2718TxHash, false, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			o := new(RawJsonTransaction)

			err := o.UnmarshalJSON(tc.jsonTx)
			if tc.eip2718 {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				return
			}

			require.Equal(t, tc.txType, o.TxType())
			require.Equal(t, tc.txHash, o.TxHash())

			reSerialized, err := o.MarshalJSON()
			require.NoError(t, err)
			require.Equal(t, tc.jsonTx, reSerialized)

			data, err := o.MarshalBinary()
			if tc.supported {
				require.NoError(t, err)
				p := new(RawJsonTransaction)
				err = p.UnmarshalBinary(data)
				require.NoError(t, err)
				require.Equal(t, o, p)
			} else {
				require.Error(t, err)
				require.Nil(t, data)
			}

			tx, err = o.Transaction()
			if tc.supported {
				require.NoError(t, err)
				require.Equal(t, tc.txHash, tx.Hash())
			} else {
				require.Error(t, err)
				require.Nil(t, tx)
			}
		})
	}
}
