package attributes

import (
	"bytes"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum-optimism/optimism/op-service/eth"
)

// AttributesMatchBlock checks if the L2 attributes pre-inputs match the output
// nil if it is a match. If err is not nil, the error contains the reason for the mismatch
func AttributesMatchBlock(rollupCfg *rollup.Config, attrs *eth.PayloadAttributes, parentHash common.Hash, envelope *eth.ExecutionPayloadEnvelope, l log.Logger) error {
	block := envelope.ExecutionPayload

	if parentHash != block.ParentHash {
		return fmt.Errorf("parent hash field does not match. expected: %v. got: %v", parentHash, block.ParentHash)
	}
	if attrs.Timestamp != block.Timestamp {
		return fmt.Errorf("timestamp field does not match. expected: %v. got: %v", uint64(attrs.Timestamp), block.Timestamp)
	}
	if attrs.PrevRandao != block.PrevRandao {
		return fmt.Errorf("random field does not match. expected: %v. got: %v", attrs.PrevRandao, block.PrevRandao)
	}
	if len(attrs.Transactions) != len(block.Transactions) {
		missingSafeHashes, missingUnsafeHashes, err := getMissingTxnHashes(l, attrs.Transactions, block.Transactions)
		if err != nil {
			l.Error("failed to get missing txn hashes", "err", err)
		} else {
			l.Error("mismatched hashes",
				"missingSafeHashes", missingSafeHashes,
				"missingUnsafeHashes", missingUnsafeHashes,
			)
		}

		return fmt.Errorf("transaction count does not match. expected: %d. got: %d", len(attrs.Transactions), len(block.Transactions))
	}
	for i, otx := range attrs.Transactions {
		if expect := block.Transactions[i]; !bytes.Equal(otx, expect) {
			if i == 0 {
				logL1InfoTxns(rollupCfg, l, uint64(block.BlockNumber), uint64(block.Timestamp), otx, block.Transactions[i])
			}
			return fmt.Errorf("transaction %d does not match. expected: %v. got: %v", i, expect, otx)
		}
	}
	if attrs.GasLimit == nil {
		return fmt.Errorf("expected gaslimit in attributes to not be nil, expected %d", block.GasLimit)
	}
	if *attrs.GasLimit != block.GasLimit {
		return fmt.Errorf("gas limit does not match. expected %d. got: %d", *attrs.GasLimit, block.GasLimit)
	}
	if withdrawalErr := checkWithdrawalsMatch(attrs.Withdrawals, block.Withdrawals); withdrawalErr != nil {
		return withdrawalErr
	}
	if err := checkParentBeaconBlockRootMatch(attrs.ParentBeaconBlockRoot, envelope.ParentBeaconBlockRoot); err != nil {
		return err
	}
	if attrs.SuggestedFeeRecipient != block.FeeRecipient {
		return fmt.Errorf("fee recipient data does not match, expected %s but got %s", block.FeeRecipient, attrs.SuggestedFeeRecipient)
	}
	return nil
}

func checkParentBeaconBlockRootMatch(attrRoot, blockRoot *common.Hash) error {
	if blockRoot == nil {
		if attrRoot != nil {
			return fmt.Errorf("expected non-nil parent beacon block root %s but got nil", *attrRoot)
		}
	} else {
		if attrRoot == nil {
			return fmt.Errorf("expected nil parent beacon block root but got non-nil %s", *blockRoot)
		} else if *blockRoot != *attrRoot {
			return fmt.Errorf("parent beacon block root does not match. expected %s. got: %s", *attrRoot, *blockRoot)
		}
	}
	return nil
}

func checkWithdrawalsMatch(attrWithdrawals *types.Withdrawals, blockWithdrawals *types.Withdrawals) error {
	if attrWithdrawals == nil && blockWithdrawals == nil {
		return nil
	}

	if attrWithdrawals == nil && blockWithdrawals != nil {
		return fmt.Errorf("expected withdrawals in block to be nil, actual %v", *blockWithdrawals)
	}

	if attrWithdrawals != nil && blockWithdrawals == nil {
		return fmt.Errorf("expected withdrawals in block to be non-nil %v, actual nil", *attrWithdrawals)
	}

	if len(*attrWithdrawals) != len(*blockWithdrawals) {
		return fmt.Errorf("expected withdrawals in block to be %d, actual %d", len(*attrWithdrawals), len(*blockWithdrawals))
	}

	for idx, expected := range *attrWithdrawals {
		actual := (*blockWithdrawals)[idx]

		if *expected != *actual {
			return fmt.Errorf("expected withdrawal %d to be %v, actual %v", idx, expected, actual)
		}
	}

	return nil
}

// logL1InfoTxns reports the values from the L1 info tx when they differ to aid
// debugging. This check is the one that has been most frequently triggered.
func logL1InfoTxns(rollupCfg *rollup.Config, l log.Logger, l2Number, l2Timestamp uint64, safeTx, unsafeTx hexutil.Bytes) {
	// First decode into *types.Transaction to get the tx data.
	var safeTxValue, unsafeTxValue types.Transaction
	errSafe := (&safeTxValue).UnmarshalBinary(safeTx)
	errUnsafe := (&unsafeTxValue).UnmarshalBinary(unsafeTx)
	if errSafe != nil || errUnsafe != nil {
		l.Error("failed to umarshal tx", "errSafe", errSafe, "errUnsafe", errUnsafe)
		return
	}

	// Then decode the ABI encoded parameters
	safeInfo, errSafe := derive.L1BlockInfoFromBytes(rollupCfg, l2Timestamp, safeTxValue.Data())
	unsafeInfo, errUnsafe := derive.L1BlockInfoFromBytes(rollupCfg, l2Timestamp, unsafeTxValue.Data())
	if errSafe != nil || errUnsafe != nil {
		l.Error("failed to umarshal l1 info", "errSafe", errSafe, "errUnsafe", errUnsafe)
		return
	}

	l = l.New("number", l2Number, "time", l2Timestamp,
		"safe_l1_number", safeInfo.Number, "safe_l1_hash", safeInfo.BlockHash,
		"safe_l1_time", safeInfo.Time, "safe_seq_num", safeInfo.SequenceNumber,
		"safe_l1_basefee", safeInfo.BaseFee, "safe_batcher_addr", safeInfo.BatcherAddr,
		"unsafe_l1_number", unsafeInfo.Number, "unsafe_l1_hash", unsafeInfo.BlockHash,
		"unsafe_l1_time", unsafeInfo.Time, "unsafe_seq_num", unsafeInfo.SequenceNumber,
		"unsafe_l1_basefee", unsafeInfo.BaseFee, "unsafe_batcher_addr", unsafeInfo.BatcherAddr,
	)
	if bytes.HasPrefix(safeTxValue.Data(), types.EcotoneL1AttributesSelector[:]) ||
		bytes.HasPrefix(safeTxValue.Data(), types.JovianL1AttributesSelector[:]) ||
		bytes.HasPrefix(safeTxValue.Data(), types.InteropL1AttributesSelector[:]) {
		l.Error("L1 Info transaction differs",
			"safe_l1_blob_basefee", safeInfo.BlobBaseFee,
			"safe_l1_basefee_scalar", safeInfo.BaseFeeScalar,
			"safe_l1_blob_basefee_scalar", safeInfo.BlobBaseFeeScalar,
			"unsafe_l1_blob_basefee", unsafeInfo.BlobBaseFee,
			"unsafe_l1_basefee_scalar", unsafeInfo.BaseFeeScalar,
			"unsafe_l1_blob_basefee_scalar", unsafeInfo.BlobBaseFeeScalar)
	} else {
		l.Error("L1 Info transaction differs",
			"safe_gpo_scalar", safeInfo.L1FeeScalar, "safe_gpo_overhead", safeInfo.L1FeeOverhead,
			"unsafe_gpo_scalar", unsafeInfo.L1FeeScalar, "unsafe_gpo_overhead", unsafeInfo.L1FeeOverhead)
	}
}

func getMissingTxnHashes(l log.Logger, safeTxns, unsafeTxns []hexutil.Bytes) ([]common.Hash, []common.Hash, error) {
	safeTxnHashes := make(map[common.Hash]struct{}, len(safeTxns))
	unsafeTxnHashes := make(map[common.Hash]struct{}, len(unsafeTxns))

	for _, tx := range safeTxns {
		safeTxValue := &types.Transaction{}
		errSafe := safeTxValue.UnmarshalBinary(tx)
		if errSafe != nil {
			return nil, nil, fmt.Errorf("failed to unmarshal safe tx: %w", errSafe)
		}

		if _, ok := safeTxnHashes[safeTxValue.Hash()]; ok {
			l.Warn("duplicate safe tx value hash detected", "safeTxValueHash", safeTxValue.Hash())
		}
		safeTxnHashes[safeTxValue.Hash()] = struct{}{}
	}

	for _, tx := range unsafeTxns {
		unsafeTxValue := &types.Transaction{}
		errUnsafe := unsafeTxValue.UnmarshalBinary(tx)
		if errUnsafe != nil {
			return nil, nil, fmt.Errorf("failed to unmarshal unsafe tx: %w", errUnsafe)
		}

		if _, ok := unsafeTxnHashes[unsafeTxValue.Hash()]; ok {
			l.Warn("duplicate unsafe tx value hash detected", "unsafeTxValueHash", unsafeTxValue.Hash())
		}
		unsafeTxnHashes[unsafeTxValue.Hash()] = struct{}{}
	}

	missingUnsafeHashes := []common.Hash{}
	for hash := range safeTxnHashes {
		if _, ok := unsafeTxnHashes[hash]; !ok {
			missingUnsafeHashes = append(missingUnsafeHashes, hash)
		}
	}

	missingSafeHashes := []common.Hash{}
	for hash := range unsafeTxnHashes {
		if _, ok := safeTxnHashes[hash]; !ok {
			missingSafeHashes = append(missingSafeHashes, hash)
		}
	}

	return missingSafeHashes, missingUnsafeHashes, nil
}
