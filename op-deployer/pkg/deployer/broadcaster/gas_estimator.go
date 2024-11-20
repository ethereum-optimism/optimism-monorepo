package broadcaster

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum-optimism/optimism/op-service/txmgr"
)

var (
	// baseFeePadFactor = 50% as a divisor
	baseFeePadFactor = big.NewInt(2)
	// tipMulFactor = 5 as a multiplier
	tipMulFactor = big.NewInt(5)
	// dummyBlobFee is a dummy value for the blob fee. Since this gas estimator will never
	// post blobs, it's just set to 1.
	dummyBlobFee = big.NewInt(1)
	// maxTip is the maximum tip that can be suggested by this estimator.
	maxTip = big.NewInt(50 * 1e9)
	// minTip is the minimum tip that can be suggested by this estimator.
	minTip = big.NewInt(1 * 1e9)
)

// DeployerGasPriceEstimator is a custom gas price estimator for use with op-deployer.
// It pads the base fee by 50% and multiplies the suggested tip by 5 up to a max of
// 50 gwei.
func DeployerGasPriceEstimator(ctx context.Context, client txmgr.ETHBackend) (*big.Int, *big.Int, *big.Int, error) {
	chainHead, err := client.HeaderByNumber(ctx, nil)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get block: %w", err)
	}

	tip, err := client.SuggestGasTipCap(ctx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get gas tip cap: %w", err)
	}

	baseFeePad := new(big.Int).Div(chainHead.BaseFee, baseFeePadFactor)
	paddedBaseFee := new(big.Int).Add(chainHead.BaseFee, baseFeePad)
	paddedTip := new(big.Int).Mul(tip, tipMulFactor)

	if paddedTip.Cmp(minTip) < 0 {
		paddedTip.Set(minTip)
	}

	if paddedTip.Cmp(maxTip) > 0 {
		paddedTip.Set(maxTip)
	}

	return paddedTip, paddedBaseFee, dummyBlobFee, nil
}
