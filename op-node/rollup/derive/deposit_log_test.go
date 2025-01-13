package derive

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-service/testutils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestUnmarshalLogEvent(t *testing.T) {
	// t.Skip("not working because deposit_log_create not working properly")
	for i := int64(0); i < 100; i++ {
		t.Run(fmt.Sprintf("random_deposit_%d", i), func(t *testing.T) {
			rng := rand.New(rand.NewSource(1234 + i))
			source := UserDepositSource{
				L1BlockHash: testutils.RandomHash(rng),
				LogIndex:    uint64(rng.Intn(10000)),
			}
			depInput := testutils.GenerateDeposit(source.SourceHash(), rng)
			log, err := MarshalDepositLogEvent(MockDepositContractAddr, depInput)
			if err != nil {
				t.Fatal(err)
			}

			log.TxIndex = uint(rng.Intn(10000))
			log.Index = uint(source.LogIndex)
			log.BlockHash = source.L1BlockHash
			depOutput, _, err := UnmarshalDepositLogEvent(log, 0)
			if err != nil {
				t.Fatal(err)
			}
			assert.Equal(t, depInput, depOutput)
		})
	}
}

// DeriveL1InfoDeposit is tested in reading_test.go, combined with the inverse ParseL1InfoDepositTxData

// receiptData defines what a test receipt looks like
type receiptData struct {
	// false = failed tx
	goodReceipt bool
	// false = not a deposit log
	DepositLogs []bool
}

func makeReceipts(rng *rand.Rand, blockHash common.Hash, depositContractAddr common.Address, testReceipts []receiptData) ([]*types.Receipt, []*types.DepositTx, error) {
	logIndex := uint(0)
	receipts := []*types.Receipt{}
	expectedDeposits := []*types.DepositTx{}
	for txIndex, rData := range testReceipts {
		var logs []*types.Log
		status := types.ReceiptStatusSuccessful
		if !rData.goodReceipt {
			status = types.ReceiptStatusFailed
		}
		for _, isDeposit := range rData.DepositLogs {
			var ev *types.Log
			var err error
			if isDeposit {
				source := UserDepositSource{L1BlockHash: blockHash, LogIndex: uint64(logIndex)}
				dep := testutils.GenerateDeposit(source.SourceHash(), rng)
				if status == types.ReceiptStatusSuccessful {
					expectedDeposits = append(expectedDeposits, dep)
				}
				ev, err = MarshalDepositLogEvent(depositContractAddr, dep)
				if err != nil {
					return []*types.Receipt{}, []*types.DepositTx{}, err
				}
			} else {
				ev = testutils.GenerateLog(testutils.RandomAddress(rng), nil, nil)
			}
			ev.TxIndex = uint(txIndex)
			ev.Index = logIndex
			ev.BlockHash = blockHash
			logs = append(logs, ev)
			logIndex++
		}

		receipts = append(receipts, &types.Receipt{
			Type:             types.DynamicFeeTxType,
			Status:           status,
			Logs:             logs,
			BlockHash:        blockHash,
			TransactionIndex: uint(txIndex),
		})
	}
	return receipts, expectedDeposits, nil
}

type DeriveUserDepositsTestCase struct {
	name string
	// generate len(receipts) receipts
	receipts []receiptData
}

func TestDeriveUserDeposits(t *testing.T) {
	// t.Skip("not working because deposit_log_create not working properly")
	testCases := []DeriveUserDepositsTestCase{
		{"no deposits", []receiptData{}},
		{"other log", []receiptData{{true, []bool{false}}}},
		{"success deposit", []receiptData{{true, []bool{true}}}},
		{"failed deposit", []receiptData{{false, []bool{true}}}},
		{"mixed deposits", []receiptData{{true, []bool{true}}, {false, []bool{true}}}},
		{"success multiple logs", []receiptData{{true, []bool{true, true}}}},
		{"failed multiple logs", []receiptData{{false, []bool{true, true}}}},
		{"not all deposit logs", []receiptData{{true, []bool{true, false, true}}}},
		{"random", []receiptData{{true, []bool{false, false, true}}, {false, []bool{}}, {true, []bool{true}}}},
	}
	for i, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			rng := rand.New(rand.NewSource(1234 + int64(i)))
			blockHash := testutils.RandomHash(rng)
			receipts, expectedDeposits, err := makeReceipts(rng, blockHash, MockDepositContractAddr, testCase.receipts)
			if err != nil {
				t.Fatal(err)
			}
			got, _, err := UserDeposits(receipts, MockDepositContractAddr, 0)
			require.NoError(t, err)
			require.Equal(t, len(got), len(expectedDeposits))
			for d, depTx := range got {
				expected := expectedDeposits[d]
				require.Equal(t, expected, depTx)
			}
		})
	}
}
