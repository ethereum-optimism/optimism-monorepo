package batcher

import (
	"context"
	"math/big"
	"time"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
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

	// Channels built for blobs have higher capacity than channels built for calldata.
	// If we have a channel built for calldata, we want to switch to blobs if the cost per byte is lower. Doing so
	// will mean a new channel is built which will not be full but will eventually fill up with additional data.
	// If we have a channel built for blobs, we similarly want to switch to calldata if the cost per byte is lower. Doing so
	// will mean several new (full) channels will be built resulting in several calldata txs. We compute the cost per byte
	// for a _single_ transaction in either case.

	// We assume that compressed random channel data has few zeros so they can be ignored (in actuality,
	// zero bytes are worth one token instead of four):
	calldataBytesPerTx := dec.calldataConfig.MaxFrameSize + 1 // +1 for the version byte
	tokensPerCalldataTx := uint64(calldataBytesPerTx * 4)
	numBlobsPerTx := dec.blobConfig.TargetNumFrames

	// Compute the total absolute cost of submitting either a single calldata tx or a single blob tx.
	calldataCost, blobCost := computeSingleCalldataTxCost(tokensPerCalldataTx, baseFee, tipCap, isPectra),
		computeSingleBlobTxCost(numBlobsPerTx, baseFee, tipCap, blobBaseFee)

	// Now we compare the absolute cost per tx divided by the number of bytes per tx:
	blobDataBytesPerTx := big.NewInt(eth.MaxBlobDataSize * int64(numBlobsPerTx))

	// The following will compare blobCost(a)/blobDataBytes(x) > calldataCost(b)/calldataBytes(y):
	ay := new(big.Int).Mul(blobCost, big.NewInt(int64(calldataBytesPerTx)))
	bx := new(big.Int).Mul(calldataCost, blobDataBytesPerTx)

	// ratio only used for logging, more correct multiplicative calculation used for comparison
	ayf, bxf := new(big.Float).SetInt(ay), new(big.Float).SetInt(bx)
	costRatio := new(big.Float).Quo(ayf, bxf)
	lgr := dec.log.New("base_fee", baseFee, "blob_base_fee", blobBaseFee, "tip_cap", tipCap,
		"calldata_bytes", calldataBytesPerTx, "calldata_cost", calldataCost,
		"blob_data_bytes", blobDataBytesPerTx, "blob_cost", blobCost,
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

func computeSingleCalldataTxCost(numTokens uint64, baseFee, tipCap *big.Int, isPectra bool) *big.Int {
	// We assume isContractCreation = false and execution_gas_used = 0 in https://eips.ethereum.org/EIPS/eip-7623
	// This is a safe assumption given how batcher transactions are constructed.
	const (
		standardTokenCost      = 4
		totalCostFloorPerToken = 10
	)
	var multiplier uint64
	if isPectra {
		multiplier = totalCostFloorPerToken
	} else {
		multiplier = standardTokenCost
	}

	calldataPrice := new(big.Int).Add(baseFee, tipCap)
	calldataGas := big.NewInt(int64(params.TxGas + numTokens*multiplier))

	return new(big.Int).Mul(calldataGas, calldataPrice)
}

func computeSingleBlobTxCost(numBlobs int, baseFee, tipCap, blobBaseFee *big.Int) *big.Int {
	// There is no execution gas or contract creation cost for blob transactions
	calldataPrice := new(big.Int).Add(baseFee, tipCap)
	blobCalldataCost := new(big.Int).Mul(big.NewInt(int64(params.TxGas)), calldataPrice)

	blobGas := big.NewInt(params.BlobTxBlobGasPerBlob * int64(numBlobs))
	blobCost := new(big.Int).Mul(blobGas, blobBaseFee)

	return blobCost.Add(blobCost, blobCalldataCost)
}
