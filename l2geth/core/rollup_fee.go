package core

import (
	"math/big"

	"github.com/ethereum/go-ethereum/params"
)

// RollupBaseTxSize is the encoded rollup transaction's compressed size excluding
// the variable length data.
// Ref: https://github.com/ethereum-optimism/optimism/blob/91a9a3dcddf534ae1c906133b6d8e015a23c463b/packages/contracts/contracts/optimistic-ethereum/OVM/predeploys/OVM_SequencerEntrypoint.sol#L47
const RollupBaseTxSize uint64 = 96

// CalculateFee calculates the fee that must be paid to the Rollup sequencer, taking into
// account the cost of publishing data to L1.
// Returns: (4 * zeroDataBytes + 16 * (nonZeroDataBytes + RollupBaseTxSize)) * dataPrice + executionPrice * gasUsed
func CalculateRollupFee(data []byte, gasUsed uint64, dataPrice, executionPrice *big.Int) *big.Int {
    var zeros uint64
    for _, byt := range data {
        if byt != 0 {
            zeros++
        }
    }
    ones := uint64(len(data)) - zeros

    zerosCost := big.NewInt(int64(zeros * params.TxDataZeroGas))
    onesCost := big.NewInt(int64((RollupBaseTxSize + ones) * params.TxDataNonZeroGasEIP2028))
    dataCost := new(big.Int).Add(zerosCost, onesCost)

	// get the data fee
	dataFee := new(big.Int).Mul(dataPrice, dataCost)
	executionFee := new(big.Int).Mul(executionPrice, new(big.Int).SetUint64(gasUsed))
	fee := new(big.Int).Add(dataFee, executionFee)
	return fee
}
