package derive

import (
	"math/big"
	"testing"

	"github.com/ethereum-optimism/optimism/op-service/testutils"
	"github.com/ethereum-optimism/optimism/op-service/testutils/fuzzerutils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	fuzz "github.com/google/gofuzz"
	"github.com/stretchr/testify/require"
)

// fuzzReceipts is similar to makeReceipts except it uses the fuzzer to populate DepositTx fields.
func fuzzReceipts(typeProvider *fuzz.Fuzzer, blockHash common.Hash, depositContractAddr common.Address) (receipts []*types.Receipt, expectedDeposits []*types.DepositTx) {
	// Determine how many receipts to generate (capped)
	var receiptCount uint64
	typeProvider.Fuzz(&receiptCount)

	// Cap our receipt count otherwise we might generate for too long and our fuzzer will assume we hung
	if receiptCount > 0x10 {
		receiptCount = 0x10
	}

	// Create every receipt we intend to
	logIndex := uint(0)
	for i := uint64(0); i < receiptCount; i++ {
		// Obtain our fuzz parameters for generating this receipt
		var txReceiptValues struct {
			GoodReceipt bool
			DepositLogs []bool
		}
		typeProvider.Fuzz(&txReceiptValues)

		// Generate a list of transaction receipts
		var logs []*types.Log
		status := types.ReceiptStatusSuccessful
		if txReceiptValues.GoodReceipt {
			status = types.ReceiptStatusFailed
		}

		// Determine if this log will be a deposit log or not and generate it accordingly
		for _, isDeposit := range txReceiptValues.DepositLogs {
			var ev *types.Log
			var err error
			if isDeposit {
				// Generate a user deposit source
				source := UserDepositSource{L1BlockHash: blockHash, LogIndex: uint64(logIndex)}

				// Fuzz parameters to construct our deposit log
				var fuzzedDepositInfo struct {
					FromAddr *common.Address
					ToAddr   *common.Address
					Value    *big.Int
					Gas      uint64
					Data     []byte
					Mint     *big.Int
				}
				typeProvider.Fuzz(&fuzzedDepositInfo)

				// Create our deposit transaction
				dep := &types.DepositTx{
					SourceHash:          source.SourceHash(),
					From:                *fuzzedDepositInfo.FromAddr,
					To:                  fuzzedDepositInfo.ToAddr,
					Value:               fuzzedDepositInfo.Value,
					Gas:                 fuzzedDepositInfo.Gas,
					Data:                fuzzedDepositInfo.Data,
					Mint:                fuzzedDepositInfo.Mint,
					IsSystemTransaction: false,
				}

				// Marshal our actual log event
				ev, err = MarshalDepositLogEvent(depositContractAddr, dep)
				if err != nil {
					panic(err)
				}

				// If we have a good version and our tx succeeded, we add this to our list of expected deposits to
				// return.
				if status == types.ReceiptStatusSuccessful {
					expectedDeposits = append(expectedDeposits, dep)
				}
			} else {
				// If we're generated an unrelated log event (not deposit), fuzz some random parameters to use.
				var randomUnrelatedLogInfo struct {
					Addr   *common.Address
					Topics []common.Hash
					Data   []byte
				}
				typeProvider.Fuzz(&randomUnrelatedLogInfo)

				// Generate the random log
				ev = testutils.GenerateLog(*randomUnrelatedLogInfo.Addr, randomUnrelatedLogInfo.Topics, randomUnrelatedLogInfo.Data)
			}
			ev.TxIndex = uint(i)
			ev.Index = logIndex
			ev.BlockHash = blockHash
			logs = append(logs, ev)
			logIndex++
		}

		// Add our receipt to our list
		receipts = append(receipts, &types.Receipt{
			Type:             types.DynamicFeeTxType,
			Status:           status,
			Logs:             logs,
			BlockHash:        blockHash,
			TransactionIndex: uint(i),
		})
	}
	return
}

// FuzzDeriveDepositsRoundTrip tests the derivation of deposits from transaction receipt event logs. It mixes
// valid and invalid deposit transactions and ensures all valid deposits are derived as expected.
// This is a fuzz test corresponding to TestDeriveUserDeposits.
func FuzzDeriveDepositsRoundTrip(f *testing.F) {
	f.Fuzz(func(t *testing.T, fuzzedData []byte) {
		// Create our fuzzer wrapper to generate complex values
		typeProvider := fuzz.NewFromGoFuzz(fuzzedData).NilChance(0).MaxDepth(10000).NumElements(0, 0x100).Funcs(
			func(e *big.Int, c fuzz.Continue) {
				var temp [32]byte
				c.Fuzz(&temp)
				e.SetBytes(temp[:])
			},
			func(e *common.Hash, c fuzz.Continue) {
				var temp [32]byte
				c.Fuzz(&temp)
				e.SetBytes(temp[:])
			},
			func(e *common.Address, c fuzz.Continue) {
				var temp [20]byte
				c.Fuzz(&temp)
				e.SetBytes(temp[:])
			})

		// Create a dummy block hash for this block
		var blockHash common.Hash
		typeProvider.Fuzz(&blockHash)

		// Fuzz to generate some random deposit events
		receipts, expectedDeposits := fuzzReceipts(typeProvider, blockHash, MockDepositContractAddr)

		// Derive our user deposits from the transaction receipts
		derivedDeposits, _, err := UserDeposits(receipts, MockDepositContractAddr, 0)
		require.NoError(t, err)

		// Ensure all deposits we derived matched what we expected to receive.
		require.Equal(t, len(derivedDeposits), len(expectedDeposits))
		for i, derivedDeposit := range derivedDeposits {
			expectedDeposit := expectedDeposits[i]
			require.Equal(t, expectedDeposit, derivedDeposit)
		}
	})
}

// FuzzDeriveDepositsBadVersion ensures that if a deposit transaction receipt event log specifies an invalid deposit
// version, no deposits should be derived.
func FuzzDeriveDepositsBadVersion(f *testing.F) {
	f.Fuzz(func(t *testing.T, fuzzedData []byte) {
		// Create our fuzzer wrapper to generate complex values
		typeProvider := fuzz.NewFromGoFuzz(fuzzedData).NilChance(0).MaxDepth(10000).NumElements(0, 0x100)
		fuzzerutils.AddFuzzerFunctions(typeProvider)

		// Create a dummy block hash for this block
		var blockHash common.Hash
		typeProvider.Fuzz(&blockHash)

		// Fuzz to generate some random deposit events
		receipts, _ := fuzzReceipts(typeProvider, blockHash, MockDepositContractAddr)

		// Loop through all receipt logs and let the fuzzer determine which (if any) to patch.
		hasBadDepositVersion := false
		for _, receipt := range receipts {

			// TODO: Using a hardcoded index (Topics[3]) here is not ideal. The MarshalDepositLogEvent method should
			//  be spliced apart to be more configurable for these tests.

			// Loop for each log in this receipt and check if it has a deposit event from our contract
			for _, log := range receipt.Logs {
				if log.Address == MockDepositContractAddr && len(log.Topics) >= 4 && log.Topics[0] == DepositEventABIHash {
					// Determine if we should set a bad deposit version for this log
					var patchBadDeposit bool
					typeProvider.Fuzz(&patchBadDeposit)
					if patchBadDeposit {
						// Generate any topic but the deposit event versions we support.
						// TODO: As opposed to keeping this hardcoded, a method such as IsValidVersion(v) should be
						//  used here.
						badVersion := DepositEventVersion0
						for badVersion == DepositEventVersion0 {
							typeProvider.Fuzz(&badVersion)
						}

						// Set our bad topic and update our state
						log.Topics[3] = PackNonceAndVersion(0, badVersion)
						hasBadDepositVersion = true
					}
				}
			}
		}

		// Derive our user deposits from the transaction receipts
		_, _, err := UserDeposits(receipts, MockDepositContractAddr, 0)

		// If we patched a bad deposit version this iteration, we should expect an error and not be able to proceed
		// further
		if hasBadDepositVersion {
			require.Errorf(t, err, "")
			return
		}
		require.NoError(t, err, "")
	})
}
