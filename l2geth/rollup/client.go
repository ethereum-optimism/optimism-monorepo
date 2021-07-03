package rollup

import (
	"errors"
	"fmt"
	"math/big"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/go-resty/resty/v2"
)

// Constants that are used to compare against values in the deserialized JSON
// fetched by the RollupClient
const (
	sequencer = "sequencer"
	l1        = "l1"
)

// errElementNotFound represents the error case of the remote element not being
// found. It applies to transactions, queue elements and batches
var errElementNotFound = errors.New("element not found")

// errHttpError represents the error case of when the remote server
// returns a 400 or greater error
var errHTTPError = errors.New("http error")

// Batch represents the data structure that is submitted with
// a series of transactions to layer one
type Batch struct {
	Index             uint64         `json:"index"`
	Root              common.Hash    `json:"root,omitempty"`
	Size              uint32         `json:"size,omitempty"`
	PrevTotalElements uint32         `json:"prevTotalElements,omitempty"`
	ExtraData         hexutil.Bytes  `json:"extraData,omitempty"`
	BlockNumber       uint64         `json:"blockNumber"`
	Timestamp         uint64         `json:"timestamp"`
	Submitter         common.Address `json:"submitter"`
}

// EthContext represents the L1 EVM context that is injected into
// the OVM at runtime. It is updated with each `enqueue` transaction
// and needs to be fetched from a remote server to be updated when
// too much time has passed between `enqueue` transactions.
type EthContext struct {
	BlockNumber uint64      `json:"blockNumber"`
	BlockHash   common.Hash `json:"blockHash"`
	Timestamp   uint64      `json:"timestamp"`
}

// SyncStatus represents the state of the remote server. The SyncService
// does not want to begin syncing until the remote server has fully synced.
type SyncStatus struct {
	Syncing                      bool   `json:"syncing"`
	HighestKnownTransactionIndex uint64 `json:"highestKnownTransactionIndex"`
	CurrentTransactionIndex      uint64 `json:"currentTransactionIndex"`
}

// L1GasPrice represents the gas price of L1. It is used as part of the gas
// estimatation logic.
type L1GasPrice struct {
	GasPrice string `json:"gasPrice"`
}

// transaction represents the return result of the remote server.
// It either came from a batch or was replicated from the sequencer.
type transaction struct {
	Index       uint64          `json:"index"`
	BatchIndex  uint64          `json:"batchIndex"`
	BlockNumber uint64          `json:"blockNumber"`
	Timestamp   uint64          `json:"timestamp"`
	Value       hexutil.Uint64  `json:"value"`
	GasLimit    uint64          `json:"gasLimit,string"`
	Target      common.Address  `json:"target"`
	Origin      *common.Address `json:"origin"`
	Data        hexutil.Bytes   `json:"data"`
	QueueOrigin string          `json:"queueOrigin"`
	QueueIndex  *uint64         `json:"queueIndex"`
	Decoded     *decoded        `json:"decoded"`
}

// Enqueue represents an `enqueue` transaction or a L1 to L2 transaction.
type Enqueue struct {
	Index       *uint64         `json:"ctcIndex"`
	Target      *common.Address `json:"target"`
	Data        *hexutil.Bytes  `json:"data"`
	GasLimit    *uint64         `json:"gasLimit,string"`
	Origin      *common.Address `json:"origin"`
	BlockNumber *uint64         `json:"blockNumber"`
	Timestamp   *uint64         `json:"timestamp"`
	QueueIndex  *uint64         `json:"index"`
}

// signature represents a secp256k1 ECDSA signature
type signature struct {
	R hexutil.Bytes `json:"r"`
	S hexutil.Bytes `json:"s"`
	V uint          `json:"v"`
}

// decoded represents the decoded transaction from the batch.
// When this struct exists in other structs and is set to `nil`,
// it means that the decoding failed.
type decoded struct {
	Signature signature       `json:"sig"`
	Value     hexutil.Uint64  `json:"value"`
	GasLimit  uint64          `json:"gasLimit,string"`
	GasPrice  uint64          `json:"gasPrice,string"`
	Nonce     uint64          `json:"nonce,string"`
	Target    *common.Address `json:"target"`
	Data      hexutil.Bytes   `json:"data"`
}

// RollupClient is able to query for information
// that is required by the SyncService
type RollupClient interface {
	GetEnqueue(index uint64) (*types.Transaction, error)
	GetLatestEnqueue() (*types.Transaction, error)
	GetLatestEnqueueIndex() (*uint64, error)
	GetTransaction(uint64, Backend) (*types.Transaction, error)
	GetLatestTransaction(Backend) (*types.Transaction, error)
	GetLatestTransactionIndex(Backend) (*uint64, error)
	GetEthContext(uint64) (*EthContext, error)
	GetLatestEthContext() (*EthContext, error)
	GetLastConfirmedEnqueue() (*types.Transaction, error)
	GetLatestTransactionBatch() (*Batch, []*types.Transaction, error)
	GetLatestTransactionBatchIndex() (*uint64, error)
	GetTransactionBatch(uint64) (*Batch, []*types.Transaction, error)
	SyncStatus(Backend) (*SyncStatus, error)
	GetL1GasPrice() (*big.Int, error)
}

// Client is an HTTP based RollupClient
type Client struct {
	client *resty.Client
	signer *types.EIP155Signer
}

// TransactionResponse represents the response from the remote server when
// querying transactions.
type TransactionResponse struct {
	Transaction *transaction `json:"transaction"`
	Batch       *Batch       `json:"batch"`
}

// TransactionBatchResponse represents the response from the remote server
// when querying batches.
type TransactionBatchResponse struct {
	Batch        *Batch         `json:"batch"`
	Transactions []*transaction `json:"transactions"`
}

// NewClient create a new Client given a remote HTTP url and a chain id
func NewClient(url string, chainID *big.Int) *Client {
	client := resty.New()
	client.SetHostURL(url)
	client.SetHeader("User-Agent", "sequencer")
	client.OnAfterResponse(func(c *resty.Client, r *resty.Response) error {
		statusCode := r.StatusCode()
		if statusCode >= 400 {
			method := r.Request.Method
			url := r.Request.URL
			return fmt.Errorf("%d cannot %s %s: %w", statusCode, method, url, errHTTPError)
		}
		return nil
	})
	signer := types.NewEIP155Signer(chainID)

	return &Client{
		client: client,
		signer: &signer,
	}
}

// GetEnqueue fetches an `enqueue` transaction by queue index
func (c *Client) GetEnqueue(index uint64) (*types.Transaction, error) {
	str := strconv.FormatUint(index, 10)
	response, err := c.client.R().
		SetPathParams(map[string]string{
			"index": str,
		}).
		SetResult(&Enqueue{}).
		Get("/enqueue/index/{index}")

	if err != nil {
		return nil, fmt.Errorf("cannot fetch enqueue: %w", err)
	}
	enqueue, ok := response.Result().(*Enqueue)
	if !ok {
		return nil, fmt.Errorf("Cannot fetch enqueue %d", index)
	}
	if enqueue == nil {
		return nil, fmt.Errorf("Cannot deserialize enqueue %d", index)
	}
	tx, err := enqueueToTransaction(enqueue)
	if err != nil {
		return nil, err
	}
	return tx, nil
}

// enqueueToTransaction turns an Enqueue into a types.Transaction
// so that it can be consumed by the SyncService
func enqueueToTransaction(enqueue *Enqueue) (*types.Transaction, error) {
	if enqueue == nil {
		return nil, errElementNotFound
	}
	// When the queue index is nil, is means that the enqueue'd transaction
	// does not exist.
	if enqueue.QueueIndex == nil {
		return nil, errElementNotFound
	}
	// The queue index is the nonce
	nonce := *enqueue.QueueIndex

	if enqueue.Target == nil {
		return nil, errors.New("Target not found for enqueue tx")
	}
	target := *enqueue.Target

	if enqueue.GasLimit == nil {
		return nil, errors.New("Gas limit not found for enqueue tx")
	}
	gasLimit := *enqueue.GasLimit
	if enqueue.Origin == nil {
		return nil, errors.New("Origin not found for enqueue tx")
	}
	origin := *enqueue.Origin
	if enqueue.BlockNumber == nil {
		return nil, errors.New("Blocknumber not found for enqueue tx")
	}
	blockNumber := new(big.Int).SetUint64(*enqueue.BlockNumber)
	if enqueue.Timestamp == nil {
		return nil, errors.New("Timestamp not found for enqueue tx")
	}
	timestamp := *enqueue.Timestamp

	if enqueue.Data == nil {
		return nil, errors.New("Data not found for enqueue tx")
	}
	data := *enqueue.Data

	// enqueue transactions have no value
	value := big.NewInt(0)
	tx := types.NewTransaction(nonce, target, value, gasLimit, big.NewInt(0), data)

	// The index does not get a check as it is allowed to be nil in the context
	// of an enqueue transaction that has yet to be included into the CTC
	txMeta := types.NewTransactionMeta(
		blockNumber,
		timestamp,
		&origin,
		types.QueueOriginL1ToL2,
		enqueue.Index,
		enqueue.QueueIndex,
		data,
	)
	tx.SetTransactionMeta(txMeta)

	return tx, nil
}

// GetLatestEnqueue fetches the latest `enqueue`, meaning the `enqueue`
// transaction with the greatest queue index.
func (c *Client) GetLatestEnqueue() (*types.Transaction, error) {
	response, err := c.client.R().
		SetResult(&Enqueue{}).
		Get("/enqueue/latest")

	if err != nil {
		return nil, fmt.Errorf("cannot fetch latest enqueue: %w", err)
	}
	enqueue, ok := response.Result().(*Enqueue)
	if !ok {
		return nil, errors.New("Cannot fetch latest enqueue")
	}
	tx, err := enqueueToTransaction(enqueue)
	if err != nil {
		return nil, fmt.Errorf("Cannot parse enqueue tx: %w", err)
	}
	return tx, nil
}

// GetLatestEnqueueIndex returns the latest `enqueue()` index
func (c *Client) GetLatestEnqueueIndex() (*uint64, error) {
	tx, err := c.GetLatestEnqueue()
	if err != nil {
		return nil, err
	}
	index := tx.GetMeta().QueueIndex
	if index == nil {
		return nil, errors.New("Latest queue index is nil")
	}
	return index, nil
}

// GetLatestTransactionIndex returns the latest CTC index that has been batch
// submitted or not, depending on the backend
func (c *Client) GetLatestTransactionIndex(backend Backend) (*uint64, error) {
	tx, err := c.GetLatestTransaction(backend)
	if err != nil {
		return nil, err
	}
	index := tx.GetMeta().Index
	if index == nil {
		return nil, errors.New("Latest index is nil")
	}
	return index, nil
}

// GetLatestTransactionBatchIndex returns the latest transaction batch index
func (c *Client) GetLatestTransactionBatchIndex() (*uint64, error) {
	batch, _, err := c.GetLatestTransactionBatch()
	if err != nil {
		return nil, err
	}
	index := batch.Index
	return &index, nil
}

// batchedTransactionToTransaction converts a transaction into a
// types.Transaction that can be consumed by the SyncService
func batchedTransactionToTransaction(res *transaction, signer *types.EIP155Signer) (*types.Transaction, error) {
	// `nil` transactions are not found
	if res == nil {
		return nil, errElementNotFound
	}
	// The queue origin must be either sequencer of l1, otherwise
	// it is considered an unknown queue origin and will not be processed
	var queueOrigin types.QueueOrigin
	if res.QueueOrigin == sequencer {
		queueOrigin = types.QueueOriginSequencer
	} else if res.QueueOrigin == l1 {
		queueOrigin = types.QueueOriginL1ToL2
	} else {
		// TODO: handle malformed transactions
		return nil, fmt.Errorf("Unknown queue origin: %s", res.QueueOrigin)
	}

	var tx *types.Transaction
	err := rlp.DecodeBytes(res.Data, tx)
	if err != nil {
		// TODO: handle malformed transactions
		return nil, fmt.Errorf("cannot rlp decode transaction: %w", err)
	}

	txMeta := types.NewTransactionMeta(
		new(big.Int).SetUint64(res.BlockNumber),
		res.Timestamp,
		res.Origin,
		queueOrigin,
		&res.Index,
		res.QueueIndex,
		res.Data,
	)
	tx.SetTransactionMeta(txMeta)
	return tx, nil
}

// GetTransaction will get a transaction by Canonical Transaction Chain index
func (c *Client) GetTransaction(index uint64, backend Backend) (*types.Transaction, error) {
	str := strconv.FormatUint(index, 10)
	response, err := c.client.R().
		SetPathParams(map[string]string{
			"index": str,
		}).
		SetQueryParams(map[string]string{
			"backend": backend.String(),
		}).
		SetResult(&TransactionResponse{}).
		Get("/transaction/index/{index}")

	if err != nil {
		return nil, fmt.Errorf("cannot fetch transaction: %w", err)
	}
	res, ok := response.Result().(*TransactionResponse)
	if !ok {
		return nil, fmt.Errorf("could not get tx with index %d", index)
	}
	return batchedTransactionToTransaction(res.Transaction, c.signer)
}

// GetLatestTransaction will get the latest transaction, meaning the transaction
// with the greatest Canonical Transaction Chain index
func (c *Client) GetLatestTransaction(backend Backend) (*types.Transaction, error) {
	response, err := c.client.R().
		SetResult(&TransactionResponse{}).
		SetQueryParams(map[string]string{
			"backend": backend.String(),
		}).
		Get("/transaction/latest")

	if err != nil {
		return nil, fmt.Errorf("cannot fetch latest transactions: %w", err)
	}
	res, ok := response.Result().(*TransactionResponse)
	if !ok {
		return nil, errors.New("Cannot get latest transaction")
	}

	return batchedTransactionToTransaction(res.Transaction, c.signer)
}

// GetEthContext will return the EthContext by block number
func (c *Client) GetEthContext(blockNumber uint64) (*EthContext, error) {
	str := strconv.FormatUint(blockNumber, 10)
	response, err := c.client.R().
		SetPathParams(map[string]string{
			"blocknumber": str,
		}).
		SetResult(&EthContext{}).
		Get("/eth/context/blocknumber/{blocknumber}")

	if err != nil {
		return nil, err
	}

	context, ok := response.Result().(*EthContext)
	if !ok {
		return nil, errors.New("Cannot parse EthContext")
	}
	return context, nil
}

// GetLatestEthContext will return the latest EthContext
func (c *Client) GetLatestEthContext() (*EthContext, error) {
	response, err := c.client.R().
		SetResult(&EthContext{}).
		Get("/eth/context/latest")

	if err != nil {
		return nil, fmt.Errorf("Cannot fetch eth context: %w", err)
	}

	context, ok := response.Result().(*EthContext)
	if !ok {
		return nil, errors.New("Cannot parse EthContext")
	}

	return context, nil
}

// GetLastConfirmedEnqueue will get the last `enqueue` transaction that has been
// batched up
func (c *Client) GetLastConfirmedEnqueue() (*types.Transaction, error) {
	enqueue, err := c.GetLatestEnqueue()
	if err != nil {
		return nil, fmt.Errorf("Cannot get latest enqueue: %w", err)
	}
	// This should only happen if there are no L1 to L2 transactions yet
	if enqueue == nil {
		return nil, errElementNotFound
	}
	// Work backwards looking for the first enqueue
	// to have an index, which means it has been included
	// in the canonical transaction chain.
	for {
		meta := enqueue.GetMeta()
		// The enqueue has an index so it has been confirmed
		if meta.Index != nil {
			return enqueue, nil
		}
		// There is no queue index so this is a bug
		if meta.QueueIndex == nil {
			return nil, fmt.Errorf("queue index is nil")
		}
		// No enqueue transactions have been confirmed yet
		if *meta.QueueIndex == uint64(0) {
			return nil, errElementNotFound
		}
		next, err := c.GetEnqueue(*meta.QueueIndex - 1)
		if err != nil {
			return nil, fmt.Errorf("cannot get enqueue %d: %w", *meta.Index, err)
		}
		enqueue = next
	}
}

// SyncStatus will query the remote server to determine if it is still syncing
func (c *Client) SyncStatus(backend Backend) (*SyncStatus, error) {
	response, err := c.client.R().
		SetResult(&SyncStatus{}).
		SetQueryParams(map[string]string{
			"backend": backend.String(),
		}).
		Get("/eth/syncing")

	if err != nil {
		return nil, fmt.Errorf("Cannot fetch sync status: %w", err)
	}

	status, ok := response.Result().(*SyncStatus)
	if !ok {
		return nil, fmt.Errorf("Cannot parse sync status")
	}

	return status, nil
}

// GetLatestTransactionBatch will return the latest transaction batch
func (c *Client) GetLatestTransactionBatch() (*Batch, []*types.Transaction, error) {
	response, err := c.client.R().
		SetResult(&TransactionBatchResponse{}).
		Get("/batch/transaction/latest")

	if err != nil {
		return nil, nil, errors.New("Cannot get latest transaction batch")
	}
	txBatch, ok := response.Result().(*TransactionBatchResponse)
	if !ok {
		return nil, nil, fmt.Errorf("Cannot parse transaction batch response")
	}
	return parseTransactionBatchResponse(txBatch, c.signer)
}

// GetTransactionBatch will return the transaction batch by batch index
func (c *Client) GetTransactionBatch(index uint64) (*Batch, []*types.Transaction, error) {
	str := strconv.FormatUint(index, 10)
	response, err := c.client.R().
		SetResult(&TransactionBatchResponse{}).
		SetPathParams(map[string]string{
			"index": str,
		}).
		Get("/batch/transaction/index/{index}")

	if err != nil {
		return nil, nil, fmt.Errorf("Cannot get transaction batch %d: %w", index, err)
	}
	txBatch, ok := response.Result().(*TransactionBatchResponse)
	if !ok {
		return nil, nil, fmt.Errorf("Cannot parse transaction batch response")
	}
	return parseTransactionBatchResponse(txBatch, c.signer)
}

// parseTransactionBatchResponse will turn a TransactionBatchResponse into a
// Batch and its corresponding types.Transactions
func parseTransactionBatchResponse(txBatch *TransactionBatchResponse, signer *types.EIP155Signer) (*Batch, []*types.Transaction, error) {
	if txBatch == nil || txBatch.Batch == nil {
		return nil, nil, errElementNotFound
	}
	batch := txBatch.Batch
	txs := make([]*types.Transaction, len(txBatch.Transactions))
	for i, tx := range txBatch.Transactions {
		transaction, err := batchedTransactionToTransaction(tx, signer)
		if err != nil {
			return nil, nil, fmt.Errorf("Cannot parse transaction batch: %w", err)
		}
		txs[i] = transaction
	}
	return batch, txs, nil
}

// GetL1GasPrice will return the current gas price on L1
func (c *Client) GetL1GasPrice() (*big.Int, error) {
	response, err := c.client.R().
		SetResult(&L1GasPrice{}).
		Get("/eth/gasprice")

	if err != nil {
		return nil, fmt.Errorf("Cannot fetch L1 gas price: %w", err)
	}

	gasPriceResp, ok := response.Result().(*L1GasPrice)
	if !ok {
		return nil, fmt.Errorf("Cannot parse L1 gas price response")
	}

	gasPrice, ok := new(big.Int).SetString(gasPriceResp.GasPrice, 10)
	if !ok {
		return nil, fmt.Errorf("Cannot parse response as big number")
	}

	return gasPrice, nil
}
