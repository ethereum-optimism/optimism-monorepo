package batcher

import (
	"context"
	"math/big"
	"time"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
)

// Spec parameters from https://eips.ethereum.org/EIPS/eip-7623
const (
	standardTokenCost      = 4
	totalCostFloorPerToken = 10
)

type (
	ChannelConfigProvider interface {
		ChannelConfig(isPectra bool) ChannelConfig
	}

	GasPricer interface {
		SuggestGasPriceCaps(ctx context.Context) (tipCap *big.Int, baseFee *big.Int, blobBaseFee *big.Int, err error)
	}

	DynamicEthChannelConfig struct {
		log       log.Logger
		timeout   time.Duration // query timeout
		gasPricer GasPricer

		blobConfig     ChannelConfig
		calldataConfig ChannelConfig
		lastConfig     *ChannelConfig
	}
)

func NewDynamicEthChannelConfig(lgr log.Logger,
	reqTimeout time.Duration, gasPricer GasPricer,
	blobConfig ChannelConfig, calldataConfig ChannelConfig,
) *DynamicEthChannelConfig {
	dec := &DynamicEthChannelConfig{
		log:            lgr,
		timeout:        reqTimeout,
		gasPricer:      gasPricer,
		blobConfig:     blobConfig,
		calldataConfig: calldataConfig,
	}
	// start with blob config
	dec.lastConfig = &dec.blobConfig
	return dec
}

// ChannelConfig will perform an estimate of the cost per byte for
// calldata and for blobs, given current market conditions: it will return
// the appropriate ChannelConfig depending on which is cheaper. It makes
// assumptions about the typical makeup of channel data.
func (dec *DynamicEthChannelConfig) ChannelConfig(isPectra bool) ChannelConfig {
	ctx, cancel := context.WithTimeout(context.Background(), dec.timeout)
	defer cancel()
	tipCap, baseFee, blobBaseFee, err := dec.gasPricer.SuggestGasPriceCaps(ctx)
	if err != nil {
		dec.log.Warn("Error querying gas prices, returning last config", "err", err)
		return *dec.lastConfig
	}

	// We estimate the gas costs of a calldata and blob tx under the assumption that we'd fill
	// a frame fully.
	// It is also assumed that a calldata tx would contain exactly one full frame
	// and a blob tx would contain target-num-frames many blobs.

	// We further assume that compressed random channel data has few zeros, so they can be
	// ignored in the calldata gas price estimation (in actuality zero bytes are worth one token instead of four):

	calldataBytes := dec.calldataConfig.MaxFrameSize + 1 // + 1 version byte
	numTokens := calldataBytes * 4                       // It would be nicer to use core.IntrinsicGas, but we don't have the actual data at hand.

	// Note we can ignore the possibility that the tx creates a contract (which in general contributes a specific amount to the gas calculation)
	// i.e. isContractCreation = false in https://eips.ethereum.org/EIPS/eip-7623
	// also execution_gas_used = 0 since batcher transactions do not call any contract code
	// Therefore the impact of EIP-7623 activating on the L1 DA layer simply scales part of the gas cost:
	var multiplier uint64
	if isPectra {
		multiplier = totalCostFloorPerToken // 10
	} else {
		multiplier = standardTokenCost // 4
	}

	calldataGas := big.NewInt(int64(params.TxGas + numTokens*multiplier))
	calldataPrice := new(big.Int).Add(baseFee, tipCap)
	calldataCost := new(big.Int).Mul(calldataGas, calldataPrice)

	blobGas := big.NewInt(params.BlobTxBlobGasPerBlob * int64(dec.blobConfig.TargetNumFrames))
	blobCost := new(big.Int).Mul(blobGas, blobBaseFee)
	// blobs still have intrinsic calldata costs
	blobCalldataCost := new(big.Int).Mul(big.NewInt(int64(params.TxGas)), calldataPrice)
	blobCost = blobCost.Add(blobCost, blobCalldataCost)

	// Now we compare the prices divided by the number of bytes that can be
	// submitted for that price.
	blobDataBytes := big.NewInt(eth.MaxBlobDataSize * int64(dec.blobConfig.TargetNumFrames))
	// The following will compare blobCost(a)/blobDataBytes(x) > calldataCost(b)/calldataBytes(y):
	ay := new(big.Int).Mul(blobCost, big.NewInt(int64(calldataBytes)))
	bx := new(big.Int).Mul(calldataCost, blobDataBytes)
	// ratio only used for logging, more correct multiplicative calculation used for comparison
	ayf, bxf := new(big.Float).SetInt(ay), new(big.Float).SetInt(bx)
	costRatio := new(big.Float).Quo(ayf, bxf)
	lgr := dec.log.New("base_fee", baseFee, "blob_base_fee", blobBaseFee, "tip_cap", tipCap,
		"calldata_bytes", calldataBytes, "calldata_cost", calldataCost,
		"blob_data_bytes", blobDataBytes, "blob_cost", blobCost,
		"cost_ratio", costRatio)

	if ay.Cmp(bx) == 1 {
		lgr.Info("Using calldata channel config")
		dec.lastConfig = &dec.calldataConfig
		return dec.calldataConfig
	}
	lgr.Info("Using blob channel config")
	dec.lastConfig = &dec.blobConfig
	return dec.blobConfig
}
