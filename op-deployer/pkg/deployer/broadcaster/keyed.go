package broadcaster

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum-optimism/optimism/op-service/eth"

	"github.com/ethereum-optimism/optimism/op-chain-ops/script"
	opcrypto "github.com/ethereum-optimism/optimism/op-service/crypto"
	"github.com/ethereum-optimism/optimism/op-service/txmgr"
	"github.com/ethereum-optimism/optimism/op-service/txmgr/metrics"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
	"github.com/hashicorp/go-multierror"
	"github.com/holiman/uint256"
)

const (
	GasPadFactor = 2.0
)

type KeyedBroadcaster struct {
	lgr    log.Logger
	mgr    txmgr.TxManager
	bcasts []script.Broadcast
	client *ethclient.Client
	mtx    sync.Mutex
}

type KeyedBroadcasterOpts struct {
	Logger  log.Logger
	ChainID *big.Int
	Client  *ethclient.Client
	Signer  opcrypto.SignerFn
	From    common.Address
}

func NewKeyedBroadcaster(cfg KeyedBroadcasterOpts) (*KeyedBroadcaster, error) {
	mgrCfg := &txmgr.Config{
		Backend:                   cfg.Client,
		ChainID:                   cfg.ChainID,
		TxSendTimeout:             5 * time.Minute,
		TxNotInMempoolTimeout:     time.Minute,
		NetworkTimeout:            10 * time.Second,
		ReceiptQueryInterval:      time.Second,
		NumConfirmations:          1,
		SafeAbortNonceTooLowCount: 3,
		Signer:                    cfg.Signer,
		From:                      cfg.From,
		GasPriceEstimatorFn:       DeployerGasPriceEstimator,
	}

	minTipCap, err := eth.GweiToWei(1.0)
	if err != nil {
		panic(err)
	}
	minBaseFee, err := eth.GweiToWei(1.0)
	if err != nil {
		panic(err)
	}

	mgrCfg.ResubmissionTimeout.Store(int64(48 * time.Second))
	mgrCfg.FeeLimitMultiplier.Store(5)
	mgrCfg.FeeLimitThreshold.Store(big.NewInt(100))
	mgrCfg.MinTipCap.Store(minTipCap)
	mgrCfg.MinBaseFee.Store(minBaseFee)

	mgr, err := txmgr.NewSimpleTxManagerFromConfig(
		"transactor",
		cfg.Logger,
		&metrics.NoopTxMetrics{},
		mgrCfg,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create tx manager: %w", err)
	}

	return &KeyedBroadcaster{
		lgr:    cfg.Logger,
		mgr:    mgr,
		client: cfg.Client,
	}, nil
}

func (t *KeyedBroadcaster) Hook(bcast script.Broadcast) {
	t.mtx.Lock()
	t.bcasts = append(t.bcasts, bcast)
	t.mtx.Unlock()
}

func (t *KeyedBroadcaster) Broadcast(ctx context.Context) ([]BroadcastResult, error) {
	// Empty the internal broadcast buffer as soon as this method is called.
	t.mtx.Lock()
	bcasts := t.bcasts
	t.bcasts = nil
	t.mtx.Unlock()

	if len(bcasts) == 0 {
		return nil, nil
	}

	results := make([]BroadcastResult, len(bcasts))
	futures := make([]<-chan txmgr.SendResponse, len(bcasts))
	ids := make([]common.Hash, len(bcasts))

	latestBlock, err := t.client.BlockByNumber(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest block: %w", err)
	}

	for i, bcast := range bcasts {
		futures[i], ids[i] = t.broadcast(ctx, bcast, latestBlock.GasLimit())
		t.lgr.Info(
			"transaction broadcasted",
			"id", ids[i],
			"nonce", bcast.Nonce,
		)
	}

	var txErr *multierror.Error
	var completed int
	for i, fut := range futures {
		bcastRes := <-fut
		completed++
		outRes := BroadcastResult{
			Broadcast: bcasts[i],
		}

		if bcastRes.Err == nil {
			outRes.Receipt = bcastRes.Receipt
			outRes.TxHash = bcastRes.Receipt.TxHash

			if bcastRes.Receipt.Status == 0 {
				failErr := fmt.Errorf("transaction failed: %s", outRes.Receipt.TxHash.String())
				txErr = multierror.Append(txErr, failErr)
				outRes.Err = failErr
				t.lgr.Error(
					"transaction failed on chain",
					"id", ids[i],
					"completed", completed,
					"total", len(bcasts),
					"hash", outRes.Receipt.TxHash.String(),
					"nonce", outRes.Broadcast.Nonce,
				)
			} else {
				t.lgr.Info(
					"transaction confirmed",
					"id", ids[i],
					"completed", completed,
					"total", len(bcasts),
					"hash", outRes.Receipt.TxHash.String(),
					"nonce", outRes.Broadcast.Nonce,
					"creation", outRes.Receipt.ContractAddress,
				)
			}
		} else {
			txErr = multierror.Append(txErr, bcastRes.Err)
			outRes.Err = bcastRes.Err
			t.lgr.Error(
				"transaction failed",
				"id", ids[i],
				"completed", completed,
				"total", len(bcasts),
				"err", bcastRes.Err,
			)
		}

		results[i] = outRes
	}
	return results, txErr.ErrorOrNil()
}

func (t *KeyedBroadcaster) broadcast(ctx context.Context, bcast script.Broadcast, blockGasLimit uint64) (<-chan txmgr.SendResponse, common.Hash) {
	ch := make(chan txmgr.SendResponse, 1)

	id := bcast.ID()
	value := ((*uint256.Int)(bcast.Value)).ToBig()
	var candidate txmgr.TxCandidate
	switch bcast.Type {
	case script.BroadcastCall:
		to := &bcast.To
		candidate = txmgr.TxCandidate{
			TxData:   bcast.Input,
			To:       to,
			Value:    value,
			GasLimit: padGasLimit(bcast.Input, bcast.GasUsed, false, blockGasLimit),
		}
	case script.BroadcastCreate:
		candidate = txmgr.TxCandidate{
			TxData:   bcast.Input,
			To:       nil,
			GasLimit: padGasLimit(bcast.Input, bcast.GasUsed, true, blockGasLimit),
		}
	case script.BroadcastCreate2:
		txData := make([]byte, len(bcast.Salt)+len(bcast.Input))
		copy(txData, bcast.Salt[:])
		copy(txData[len(bcast.Salt):], bcast.Input)

		candidate = txmgr.TxCandidate{
			TxData:   txData,
			To:       &script.DeterministicDeployerAddress,
			Value:    value,
			GasLimit: padGasLimit(bcast.Input, bcast.GasUsed, true, blockGasLimit),
		}
	}

	t.mgr.SendAsync(ctx, candidate, ch)
	return ch, id
}

// padGasLimit calculates the gas limit for a transaction based on the intrinsic gas and the gas used by
// the underlying call. Values are multiplied by a pad factor to account for any discrepancies. The output
// is clamped to the block gas limit since Geth will reject transactions that exceed it before letting them
// into the mempool.
func padGasLimit(data []byte, gasUsed uint64, creation bool, blockGasLimit uint64) uint64 {
	intrinsicGas, err := core.IntrinsicGas(data, nil, creation, true, true, false)
	// This method never errors - we should look into it if it does.
	if err != nil {
		panic(err)
	}

	limit := uint64(float64(intrinsicGas+gasUsed) * GasPadFactor)
	if limit > blockGasLimit {
		return blockGasLimit
	}
	return limit
}
