package txmgr

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/misc/eip4844"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/txpool"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
	"github.com/ethereum/go-ethereum/log"
	"github.com/holiman/uint256"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/retry"
	"github.com/ethereum-optimism/optimism/op-service/txmgr/metrics"
)

const (
	// Geth requires a minimum fee bump of 10% for regular tx resubmission, 100% for blob txs
	priceBump     int64 = 10
	blobPriceBump int64 = 100
)

// new = old * (100 + priceBump) / 100
var (
	priceBumpPercent     = big.NewInt(100 + priceBump)
	oneHundred           = big.NewInt(100)
	blobPriceBumpPercent = big.NewInt(100 + blobPriceBump)
	two                  = big.NewInt(2)
)

// TxManager is an interface that allows callers to reliably publish txs,
// bumping the gas price if needed, and obtain the receipt of the resulting tx.
//
//go:generate mockery --name TxManager --output ./mocks
type TxManager interface {
	// Send is used to create & send a transaction. It will handle increasing
	// the gas price & ensuring that the transaction remains in the transaction pool.
	// It can be stopped by cancelling the provided context; however, the transaction
	// may be included on L1 even if the context is cancelled.
	//
	// NOTE: Send can be called concurrently, the nonce will be managed internally.
	Send(ctx context.Context, candidate TxCandidate) (*types.Receipt, error)

	// From returns the sending address associated with the instance of the transaction manager.
	// It is static for a single instance of a TxManager.
	From() common.Address

	// BlockNumber returns the most recent block number from the underlying network.
	BlockNumber(ctx context.Context) (uint64, error)

	// Close the underlying connection
	Close()
}

// ETHBackend is the set of methods that the transaction manager uses to resubmit gas & determine
// when transactions are included on L1.
type ETHBackend interface {
	// BlockNumber returns the most recent block number.
	BlockNumber(ctx context.Context) (uint64, error)

	// CallContract executes an eth_call against the provided contract.
	CallContract(ctx context.Context, msg ethereum.CallMsg, blockNumber *big.Int) ([]byte, error)

	// TransactionReceipt queries the backend for a receipt associated with
	// txHash. If lookup does not fail, but the transaction is not found,
	// nil should be returned for both values.
	TransactionReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error)

	// SendTransaction submits a signed transaction to L1.
	SendTransaction(ctx context.Context, tx *types.Transaction) error

	// These functions are used to estimate what the basefee & priority fee should be set to.
	// TODO(CLI-3318): Maybe need a generic interface to support different RPC providers
	HeaderByNumber(ctx context.Context, number *big.Int) (*types.Header, error)
	SuggestGasTipCap(ctx context.Context) (*big.Int, error)
	// NonceAt returns the account nonce of the given account.
	// The block number can be nil, in which case the nonce is taken from the latest known block.
	NonceAt(ctx context.Context, account common.Address, blockNumber *big.Int) (uint64, error)
	// PendingNonceAt returns the pending nonce.
	PendingNonceAt(ctx context.Context, account common.Address) (uint64, error)
	// EstimateGas returns an estimate of the amount of gas needed to execute the given
	// transaction against the current pending block.
	EstimateGas(ctx context.Context, msg ethereum.CallMsg) (uint64, error)
	// Close the underlying eth connection
	Close()
}

// SimpleTxManager is a implementation of TxManager that performs linear fee
// bumping of a tx until it confirms.
type SimpleTxManager struct {
	cfg     Config // embed the config directly
	name    string
	chainID *big.Int

	backend ETHBackend
	l       log.Logger
	metr    metrics.TxMetricer

	nonce     *uint64
	nonceLock sync.RWMutex

	pending atomic.Int64
}

// NewSimpleTxManager initializes a new SimpleTxManager with the passed Config.
func NewSimpleTxManager(name string, l log.Logger, m metrics.TxMetricer, cfg CLIConfig) (*SimpleTxManager, error) {
	conf, err := NewConfig(cfg, l)
	if err != nil {
		return nil, err
	}
	return NewSimpleTxManagerFromConfig(name, l, m, conf)
}

// NewSimpleTxManager initializes a new SimpleTxManager with the passed Config.
func NewSimpleTxManagerFromConfig(name string, l log.Logger, m metrics.TxMetricer, conf Config) (*SimpleTxManager, error) {
	if err := conf.Check(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	return &SimpleTxManager{
		chainID: conf.ChainID,
		name:    name,
		cfg:     conf,
		backend: conf.Backend,
		l:       l.New("service", name),
		metr:    m,
	}, nil
}

func (m *SimpleTxManager) From() common.Address {
	return m.cfg.From
}

func (m *SimpleTxManager) BlockNumber(ctx context.Context) (uint64, error) {
	return m.backend.BlockNumber(ctx)
}

func (m *SimpleTxManager) Close() {
	m.backend.Close()
}

// TxCandidate is a transaction candidate that can be submitted to ask the
// [TxManager] to construct a transaction with gas price bounds.
type TxCandidate struct {
	// TxData is the transaction calldata to be used in the constructed tx.
	TxData []byte
	// Blobs to send along in the tx (optional).
	Blobs []*eth.Blob
	// To is the recipient of the constructed tx. Nil means contract creation.
	To *common.Address
	// GasLimit is the gas limit to be used in the constructed tx.
	GasLimit uint64
	// Value is the value to be used in the constructed tx.
	Value *big.Int
}

// Send is used to publish a transaction with incrementally higher gas prices
// until the transaction eventually confirms. This method blocks until an
// invocation of sendTx returns (called with differing gas prices). The method
// may be canceled using the passed context.
//
// The transaction manager handles all signing. If and only if the gas limit is 0, the
// transaction manager will do a gas estimation.
//
// NOTE: Send can be called concurrently, the nonce will be managed internally.
func (m *SimpleTxManager) Send(ctx context.Context, candidate TxCandidate) (*types.Receipt, error) {
	m.metr.RecordPendingTx(m.pending.Add(1))
	defer func() {
		m.metr.RecordPendingTx(m.pending.Add(-1))
	}()
	receipt, err := m.send(ctx, candidate)
	if err != nil {
		m.resetNonce()
	}
	return receipt, err
}

// send performs the actual transaction creation and sending.
func (m *SimpleTxManager) send(ctx context.Context, candidate TxCandidate) (*types.Receipt, error) {
	if m.cfg.TxSendTimeout != 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, m.cfg.TxSendTimeout)
		defer cancel()
	}
	tx, err := retry.Do(ctx, 30, retry.Fixed(2*time.Second), func() (*types.Transaction, error) {
		tx, err := m.craftTx(ctx, candidate)
		if err != nil {
			m.l.Warn("Failed to create a transaction, will retry", "err", err)
		}
		return tx, err
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create the tx: %w", err)
	}
	return m.sendTx(ctx, tx)
}

// craftTx creates the signed transaction
// It queries L1 for the current fee market conditions as well as for the nonce.
// NOTE: This method SHOULD NOT publish the resulting transaction.
// NOTE: If the [TxCandidate.GasLimit] is non-zero, it will be used as the transaction's gas.
// NOTE: Otherwise, the [SimpleTxManager] will query the specified backend for an estimate.
func (m *SimpleTxManager) craftTx(ctx context.Context, candidate TxCandidate) (*types.Transaction, error) {
	log := m.l.New("to", candidate.To, "from", m.cfg.From, "blobs", len(candidate.Blobs))
	log.Info("Creating tx")

	gasTipCap, basefee, blobFee, err := m.suggestGasPriceCaps(ctx)
	if err != nil {
		m.metr.RPCError()
		return nil, fmt.Errorf("failed to get gas price info: %w", err)
	}
	gasFeeCap := calcGasFeeCap(basefee, gasTipCap)

	gasLimit := candidate.GasLimit

	// If the gas limit is set, we can use that as the gas
	if gasLimit == 0 {
		// Calculate the intrinsic gas for the transaction
		gas, err := m.backend.EstimateGas(ctx, ethereum.CallMsg{
			From:      m.cfg.From,
			To:        candidate.To,
			GasFeeCap: gasFeeCap,
			Data:      candidate.TxData,
			Value:     candidate.Value,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to estimate gas: %w", err)
		}
		gasLimit = gas
		log.Info("estimated gas", "gas", gasLimit)
	}

	var sidecar *types.BlobTxSidecar
	var blobHashes []common.Hash
	if len(candidate.Blobs) > 0 {
		if candidate.To == nil {
			return nil, errors.New("blob txs cannot deploy contracts")
		}
		blobHashes = make([]common.Hash, 0, len(candidate.Blobs))
		sidecar = &types.BlobTxSidecar{}
		for i, blob := range candidate.Blobs {
			rawBlob := *blob.KZGBlob()
			sidecar.Blobs = append(sidecar.Blobs, rawBlob)
			commitment, err := kzg4844.BlobToCommitment(rawBlob)
			if err != nil {
				return nil, fmt.Errorf("cannot compute KZG commitment of blob %d in tx candidate: %w", i, err)
			}
			sidecar.Commitments = append(sidecar.Commitments, commitment)
			proof, err := kzg4844.ComputeBlobProof(rawBlob, commitment)
			if err != nil {
				return nil, fmt.Errorf("cannot compute KZG proof for fast commitment verification of blob %d in tx candidate: %w", i, err)
			}
			sidecar.Proofs = append(sidecar.Proofs, proof)
			blobHashes = append(blobHashes, eth.KZGToVersionedHash(commitment))
		}
		log.Info("crafted blob tx sidecar")
	}

	var txMessage types.TxData
	if sidecar != nil {
		if blobFee == nil {
			return nil, fmt.Errorf("expected non-nil blobFee")
		}
		blobFeeCap := calcBlobFeeCap(blobFee)
		txMessage = &types.BlobTx{
			ChainID:    uint256.MustFromBig(m.chainID),
			Nonce:      0, // set by signer
			To:         *candidate.To,
			GasTipCap:  uint256.MustFromBig(gasTipCap),
			GasFeeCap:  uint256.MustFromBig(gasFeeCap),
			Value:      uint256.MustFromBig(candidate.Value),
			Data:       candidate.TxData,
			Gas:        gasLimit,
			BlobFeeCap: uint256.MustFromBig(blobFeeCap),
			BlobHashes: blobHashes,
			Sidecar:    sidecar,
		}
	} else {
		txMessage = &types.DynamicFeeTx{
			ChainID:   m.chainID,
			Nonce:     0, // set by signer
			To:        candidate.To,
			GasTipCap: gasTipCap,
			GasFeeCap: gasFeeCap,
			Value:     candidate.Value,
			Data:      candidate.TxData,
			Gas:       gasLimit,
		}
	}

	return m.signWithNextNonce(ctx, txMessage)
}

// signWithNextNonce returns a signed transaction with the next available nonce.
// The nonce is fetched once using eth_getTransactionCount with "latest", and
// then subsequent calls simply increment this number. If the transaction manager
// is reset, it will query the eth_getTransactionCount nonce again. If signing
// fails, the nonce is not incremented.
func (m *SimpleTxManager) signWithNextNonce(ctx context.Context, txMessage types.TxData) (*types.Transaction, error) {
	m.nonceLock.Lock()
	defer m.nonceLock.Unlock()

	if m.nonce == nil {
		// Fetch the sender's nonce from the latest known block (nil `blockNumber`)
		childCtx, cancel := context.WithTimeout(ctx, m.cfg.NetworkTimeout)
		defer cancel()
		nonce, err := m.backend.NonceAt(childCtx, m.cfg.From, nil)
		if err != nil {
			m.metr.RPCError()
			return nil, fmt.Errorf("failed to get nonce: %w", err)
		}
		m.nonce = &nonce
	} else {
		*m.nonce++
	}

	switch x := txMessage.(type) {
	case *types.DynamicFeeTx:
		x.Nonce = *m.nonce
	case *types.BlobTx:
		x.Nonce = *m.nonce
	default:
		return nil, fmt.Errorf("unrecognized tx type: %T", x)
	}
	ctx, cancel := context.WithTimeout(ctx, m.cfg.NetworkTimeout)
	defer cancel()
	tx, err := m.cfg.Signer(ctx, m.cfg.From, types.NewTx(txMessage))
	if err != nil {
		// decrement the nonce, so we can retry signing with the same nonce next time
		// signWithNextNonce is called
		*m.nonce--
	} else {
		m.metr.RecordNonce(*m.nonce)
	}
	return tx, err
}

// resetNonce resets the internal nonce tracking. This is called if any pending send
// returns an error.
func (m *SimpleTxManager) resetNonce() {
	m.nonceLock.Lock()
	defer m.nonceLock.Unlock()
	m.nonce = nil
}

// send submits the same transaction several times with increasing gas prices as necessary.
// It waits for the transaction to be confirmed on chain.
func (m *SimpleTxManager) sendTx(ctx context.Context, tx *types.Transaction) (*types.Receipt, error) {
	var wg sync.WaitGroup
	defer wg.Wait()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sendState := NewSendState(m.cfg.SafeAbortNonceTooLowCount, m.cfg.TxNotInMempoolTimeout)
	receiptChan := make(chan *types.Receipt, 1)
	publishAndWait := func(tx *types.Transaction, bumpFees bool) *types.Transaction {
		wg.Add(1)
		tx, published := m.publishTx(ctx, tx, sendState, bumpFees)
		if published {
			go func() {
				defer wg.Done()
				m.waitForTx(ctx, tx, sendState, receiptChan)
			}()
		} else {
			wg.Done()
		}
		return tx
	}

	// Immediately publish a transaction before starting the resumbission loop
	tx = publishAndWait(tx, false)

	ticker := time.NewTicker(m.cfg.ResubmissionTimeout)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Don't resubmit a transaction if it has been mined, but we are waiting for the conf depth.
			if sendState.IsWaitingForConfirmation() {
				continue
			}
			// If we see lots of unrecoverable errors (and no pending transactions) abort sending the transaction.
			if sendState.ShouldAbortImmediately() {
				m.l.Warn("Aborting transaction submission")
				return nil, errors.New("aborted transaction sending")
			}
			tx = publishAndWait(tx, true)

		case <-ctx.Done():
			return nil, ctx.Err()

		case receipt := <-receiptChan:
			m.metr.RecordGasBumpCount(sendState.bumpCount)
			m.metr.TxConfirmed(receipt)
			return receipt, nil
		}
	}
}

// publishTx publishes the transaction to the transaction pool. If it receives any underpriced errors
// it will bump the fees and retry.
// Returns the latest fee bumped tx, and a boolean indicating whether the tx was sent or not
func (m *SimpleTxManager) publishTx(ctx context.Context, tx *types.Transaction, sendState *SendState, bumpFeesImmediately bool) (*types.Transaction, bool) {
	updateLogFields := func(tx *types.Transaction) log.Logger {
		logger := m.l.New("hash", tx.Hash(), "nonce", tx.Nonce(), "gasTipCap", tx.GasTipCap(), "gasFeeCap", tx.GasFeeCap())
		if sidecar := tx.BlobTxSidecar(); sidecar != nil {
			logger = logger.New("blobs", len(sidecar.Blobs))
		}
		return logger
	}
	l := updateLogFields(tx)

	l.Info("Publishing transaction")

	for {
		if bumpFeesImmediately {
			newTx, err := m.increaseGasPrice(ctx, tx)
			if err != nil {
				l.Error("unable to increase gas", "err", err)
				m.metr.TxPublished("bump_failed")
				return tx, false
			}
			tx = newTx
			sendState.bumpCount++
			l = updateLogFields(tx)
		}
		bumpFeesImmediately = true // bump fees next loop

		if sendState.IsWaitingForConfirmation() {
			// there is a chance the previous tx goes into "waiting for confirmation" state
			// during the increaseGasPrice call; continue waiting rather than resubmit the tx
			return tx, false
		}

		cCtx, cancel := context.WithTimeout(ctx, m.cfg.NetworkTimeout)
		err := m.backend.SendTransaction(cCtx, tx)
		cancel()
		sendState.ProcessSendError(err)

		if err == nil {
			m.metr.TxPublished("")
			log.Info("Transaction successfully published")
			return tx, true
		}

		switch {
		case errStringMatch(err, core.ErrNonceTooLow):
			l.Warn("nonce too low", "err", err)
			m.metr.TxPublished("nonce_to_low")
		case errStringMatch(err, context.Canceled):
			m.metr.RPCError()
			l.Warn("transaction send cancelled", "err", err)
			m.metr.TxPublished("context_cancelled")
		case errStringMatch(err, txpool.ErrAlreadyKnown):
			l.Warn("resubmitted already known transaction", "err", err)
			m.metr.TxPublished("tx_already_known")
		case errStringMatch(err, txpool.ErrReplaceUnderpriced):
			l.Warn("transaction replacement is underpriced", "err", err)
			m.metr.TxPublished("tx_replacement_underpriced")
			continue // retry with fee bump
		case errStringMatch(err, txpool.ErrUnderpriced):
			l.Warn("transaction is underpriced", "err", err)
			m.metr.TxPublished("tx_underpriced")
			continue // retry with fee bump
		default:
			m.metr.RPCError()
			l.Error("unable to publish transaction", "err", err)
			m.metr.TxPublished("unknown_error")
		}

		// on non-underpriced error return immediately; will retry on next resubmission timeout
		return tx, false
	}
}

// waitForTx calls waitMined, and then sends the receipt to receiptChan in a non-blocking way if a receipt is found
// for the transaction. It should be called in a separate goroutine.
func (m *SimpleTxManager) waitForTx(ctx context.Context, tx *types.Transaction, sendState *SendState, receiptChan chan *types.Receipt) {
	t := time.Now()
	// Poll for the transaction to be ready & then send the result to receiptChan
	receipt, err := m.waitMined(ctx, tx, sendState)
	if err != nil {
		// this will happen if the tx was successfully replaced by a tx with bumped fees
		log.Info("Transaction receipt not found", "err", err)
		return
	}
	select {
	case receiptChan <- receipt:
		m.metr.RecordTxConfirmationLatency(time.Since(t).Milliseconds())
	default:
	}
}

// waitMined waits for the transaction to be mined or for the context to be cancelled.
func (m *SimpleTxManager) waitMined(ctx context.Context, tx *types.Transaction, sendState *SendState) (*types.Receipt, error) {
	txHash := tx.Hash()
	queryTicker := time.NewTicker(m.cfg.ReceiptQueryInterval)
	defer queryTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-queryTicker.C:
			if receipt := m.queryReceipt(ctx, txHash, sendState); receipt != nil {
				return receipt, nil
			}
		}
	}
}

// queryReceipt queries for the receipt and returns the receipt if it has passed the confirmation depth
func (m *SimpleTxManager) queryReceipt(ctx context.Context, txHash common.Hash, sendState *SendState) *types.Receipt {
	ctx, cancel := context.WithTimeout(ctx, m.cfg.NetworkTimeout)
	defer cancel()
	receipt, err := m.backend.TransactionReceipt(ctx, txHash)
	if errors.Is(err, ethereum.NotFound) {
		sendState.TxNotMined(txHash)
		m.l.Trace("Transaction not yet mined", "hash", txHash)
		return nil
	} else if err != nil {
		m.metr.RPCError()
		m.l.Info("Receipt retrieval failed", "hash", txHash, "err", err)
		return nil
	} else if receipt == nil {
		m.metr.RPCError()
		m.l.Warn("Receipt and error are both nil", "hash", txHash)
		return nil
	}

	// Receipt is confirmed to be valid from this point on
	sendState.TxMined(txHash)

	txHeight := receipt.BlockNumber.Uint64()
	tipHeight, err := m.backend.BlockNumber(ctx)
	if err != nil {
		m.l.Error("Unable to fetch block number", "err", err)
		return nil
	}

	m.l.Debug("Transaction mined, checking confirmations", "hash", txHash, "txHeight", txHeight,
		"tipHeight", tipHeight, "numConfirmations", m.cfg.NumConfirmations)

	// The transaction is considered confirmed when
	// txHeight+numConfirmations-1 <= tipHeight. Note that the -1 is
	// needed to account for the fact that confirmations have an
	// inherent off-by-one, i.e. when using 1 confirmation the
	// transaction should be confirmed when txHeight is equal to
	// tipHeight. The equation is rewritten in this form to avoid
	// underflows.
	if txHeight+m.cfg.NumConfirmations <= tipHeight+1 {
		m.l.Info("Transaction confirmed", "hash", txHash)
		return receipt
	}

	// Safe to subtract since we know the LHS above is greater.
	confsRemaining := (txHeight + m.cfg.NumConfirmations) - (tipHeight + 1)
	m.l.Debug("Transaction not yet confirmed", "hash", txHash, "confsRemaining", confsRemaining)
	return nil
}

// increaseGasPrice returns a new transaction that is equivalent to the input transaction but with
// higher fees that should satisfy geth's tx replacement rules. It also computes an updated gas
// limit estimate. To avoid runaway price increases, fees are capped at a `feeLimitMultiplier`
// multiple of the suggested values.
func (m *SimpleTxManager) increaseGasPrice(ctx context.Context, tx *types.Transaction) (*types.Transaction, error) {
	m.l.Info("bumping gas price for tx", "hash", tx.Hash(), "gasTipCap", tx.GasTipCap(), "gasFeeCap", tx.GasFeeCap(), "gaslimit", tx.Gas())
	tip, basefee, blobFee, err := m.suggestGasPriceCaps(ctx)
	if err != nil {
		m.l.Warn("failed to get suggested gas tip and basefee", "err", err)
		return nil, err
	}
	bumpedTip, bumpedFee := updateFees(tx.GasTipCap(), tx.GasFeeCap(), tip, basefee, tx.Type() == types.BlobTxType, m.l)

	// Make sure increase is at most [FeeLimitMultiplier] the suggested values
	maxTip := new(big.Int).Mul(tip, big.NewInt(int64(m.cfg.FeeLimitMultiplier)))
	if bumpedTip.Cmp(maxTip) > 0 {
		return nil, fmt.Errorf("bumped tip cap %v is over %dx multiple of the suggested value", bumpedTip, m.cfg.FeeLimitMultiplier)
	}
	maxFee := calcGasFeeCap(new(big.Int).Mul(basefee, big.NewInt(int64(m.cfg.FeeLimitMultiplier))), maxTip)
	if bumpedFee.Cmp(maxFee) > 0 {
		return nil, fmt.Errorf("bumped fee cap %v is over %dx multiple of the suggested value", bumpedFee, m.cfg.FeeLimitMultiplier)
	}
	// Re-estimate gaslimit in case things have changed or a previous gaslimit estimate was wrong
	gas, err := m.backend.EstimateGas(ctx, ethereum.CallMsg{
		From:      m.cfg.From,
		To:        tx.To(),
		GasTipCap: bumpedTip,
		GasFeeCap: bumpedFee,
		Data:      tx.Data(),
		Value:     tx.Value(),
	})
	if err != nil {
		// If this is a transaction resubmission, we sometimes see this outcome because the
		// original tx can get included in a block just before the above call. In this case the
		// error is due to the tx reverting with message "block number must be equal to next
		// expected block number"
		m.l.Warn("failed to re-estimate gas", "err", err, "gaslimit", tx.Gas(),
			"gasFeeCap", bumpedFee, "gasTipCap", bumpedTip)
		return nil, err
	}
	if tx.Gas() != gas {
		m.l.Info("re-estimated gas differs", "oldgas", tx.Gas(), "newgas", gas,
			"gasFeeCap", bumpedFee, "gasTipCap", bumpedTip)
	}

	var newTx *types.Transaction
	if tx.Type() == types.BlobTxType {
		bumpedBlobFee := calcThresholdValue(tx.BlobGasFeeCap(), true)
		if bumpedBlobFee.Cmp(blobFee) < 0 {
			bumpedBlobFee = blobFee
		}
		maxBlobFee := new(big.Int).Mul(calcBlobFeeCap(blobFee), big.NewInt(int64(m.cfg.FeeLimitMultiplier)))
		if bumpedBlobFee.Cmp(maxBlobFee) > 0 {
			m.l.Warn("bumped blob fee getting capped at multiple of the suggested value", "bumped", bumpedBlobFee, "suggestion", maxBlobFee)
			bumpedBlobFee.Set(maxBlobFee)
		}
		newTx = types.NewTx(&types.BlobTx{
			ChainID:    uint256.MustFromBig(tx.ChainId()),
			Nonce:      tx.Nonce(),
			To:         *tx.To(),
			GasTipCap:  uint256.MustFromBig(bumpedTip),
			GasFeeCap:  uint256.MustFromBig(bumpedFee),
			Value:      uint256.MustFromBig(tx.Value()),
			Data:       tx.Data(),
			Gas:        gas,
			BlobFeeCap: uint256.MustFromBig(bumpedBlobFee),
			BlobHashes: tx.BlobHashes(),
			Sidecar:    tx.BlobTxSidecar(),
		})
	} else {
		newTx = types.NewTx(&types.DynamicFeeTx{
			ChainID:   tx.ChainId(),
			Nonce:     tx.Nonce(),
			To:        tx.To(),
			GasTipCap: bumpedTip,
			GasFeeCap: bumpedFee,
			Value:     tx.Value(),
			Data:      tx.Data(),
			Gas:       gas,
		})
	}

	ctx, cancel := context.WithTimeout(ctx, m.cfg.NetworkTimeout)
	defer cancel()
	signedTx, err := m.cfg.Signer(ctx, m.cfg.From, newTx)
	if err != nil {
		m.l.Warn("failed to sign new transaction", "err", err)
		return tx, nil
	}
	return signedTx, nil
}

// suggestGasPriceCaps suggests what the new tip, basefee, and blobfee should be based on the
// current L1 conditions. blobfee will be nil if 4844 is not yet active.
func (m *SimpleTxManager) suggestGasPriceCaps(ctx context.Context) (*big.Int, *big.Int, *big.Int, error) {
	cCtx, cancel := context.WithTimeout(ctx, m.cfg.NetworkTimeout)
	defer cancel()
	tip, err := m.backend.SuggestGasTipCap(cCtx)
	if err != nil {
		m.metr.RPCError()
		return nil, nil, nil, fmt.Errorf("failed to fetch the suggested gas tip cap: %w", err)
	} else if tip == nil {
		return nil, nil, nil, errors.New("the suggested tip was nil")
	}
	cCtx, cancel = context.WithTimeout(ctx, m.cfg.NetworkTimeout)
	defer cancel()
	head, err := m.backend.HeaderByNumber(cCtx, nil)
	if err != nil {
		m.metr.RPCError()
		return nil, nil, nil, fmt.Errorf("failed to fetch the suggested basefee: %w", err)
	} else if head.BaseFee == nil {
		return nil, nil, nil, errors.New("txmgr does not support pre-london blocks that do not have a basefee")
	}
	var blobFee *big.Int
	if head.ExcessBlobGas != nil {
		blobFee = eip4844.CalcBlobFee(*head.ExcessBlobGas)
	}
	return tip, head.BaseFee, blobFee, nil
}

// calcThresholdValue returns x * priceBumpPercent / 100
func calcThresholdValue(x *big.Int, isBlobTx bool) *big.Int {
	var percent *big.Int
	if isBlobTx {
		percent = blobPriceBumpPercent
	} else {
		percent = priceBumpPercent
	}
	threshold := new(big.Int).Mul(percent, x)
	threshold = threshold.Div(threshold, oneHundred)
	return threshold
}

// updateFees takes an old transaction's tip & fee cap plus a new tip & basefee, and returns
// a suggested tip and fee cap such that:
//
//	(a) each satisfies geth's required tx-replacement fee bumps, and
//	(b) gasTipCap is no less than new tip, and
//	(c) gasFeeCap is no less than calcGasFee(newBaseFee, newTip)
func updateFees(oldTip, oldFeeCap, newTip, newBaseFee *big.Int, isBlobTx bool, lgr log.Logger) (*big.Int, *big.Int) {
	newFeeCap := calcGasFeeCap(newBaseFee, newTip)
	lgr = lgr.New("old_gasTipCap", oldTip, "old_gasFeeCap", oldFeeCap,
		"new_gasTipCap", newTip, "new_gasFeeCap", newFeeCap, "new_basefee", newBaseFee)
	thresholdTip := calcThresholdValue(oldTip, isBlobTx)
	thresholdFeeCap := calcThresholdValue(oldFeeCap, isBlobTx)
	if newTip.Cmp(thresholdTip) >= 0 && newFeeCap.Cmp(thresholdFeeCap) >= 0 {
		lgr.Debug("Using new tip and feecap")
		return newTip, newFeeCap
	} else if newTip.Cmp(thresholdTip) >= 0 && newFeeCap.Cmp(thresholdFeeCap) < 0 {
		// Tip has gone up, but basefee is flat or down.
		// TODO(CLI-3714): Do we need to recalculate the FC here?
		lgr.Debug("Using new tip and threshold feecap")
		return newTip, thresholdFeeCap
	} else if newTip.Cmp(thresholdTip) < 0 && newFeeCap.Cmp(thresholdFeeCap) >= 0 {
		// Basefee has gone up, but the tip hasn't. Recalculate the feecap because if the tip went up a lot
		// not enough of the feecap may be dedicated to paying the basefee.
		lgr.Debug("Using threshold tip and recalculated feecap")
		return thresholdTip, calcGasFeeCap(newBaseFee, thresholdTip)

	} else {
		// TODO(CLI-3713): Should we skip the bump in this case?
		lgr.Debug("Using threshold tip and threshold feecap")
		return thresholdTip, thresholdFeeCap
	}
}

// calcGasFeeCap deterministically computes the recommended gas fee cap given
// the base fee and gasTipCap. The resulting gasFeeCap is equal to:
//
//	gasTipCap + 2*baseFee.
func calcGasFeeCap(baseFee, gasTipCap *big.Int) *big.Int {
	return new(big.Int).Add(
		gasTipCap,
		new(big.Int).Mul(baseFee, two),
	)
}

// calcBlobFeeCap computes a suggested blob fee cap that is twice the current header's blob fee
// value.
func calcBlobFeeCap(blobFee *big.Int) *big.Int {
	return new(big.Int).Mul(blobFee, two)
}

// errStringMatch returns true if err.Error() is a substring in target.Error() or if both are nil.
// It can accept nil errors without issue.
func errStringMatch(err, target error) bool {
	if err == nil && target == nil {
		return true
	} else if err == nil || target == nil {
		return false
	}
	return strings.Contains(err.Error(), target.Error())
}
