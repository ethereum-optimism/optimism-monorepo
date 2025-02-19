package system

import (
	"context"
	"fmt"
	"log/slog"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"
)

// Default values for gas calculations
const (
	DefaultGasLimitMarginPercent = 20 // 20% margin for gas limit
	DefaultFeeCapMultiplier      = 2  // 2x gas price for fee cap
)

// TxBuilderOption is a function that configures a TxBuilder
type TxBuilderOption func(*TxBuilder)

// WithTxType sets the transaction type to use, overriding automatic detection
func WithTxType(txType uint8) TxBuilderOption {
	return func(b *TxBuilder) {
		b.forcedTxType = &txType
		b.supportedTxTypes = []uint8{txType}
	}
}

// WithGasLimitMargin sets the margin percentage to add to estimated gas limit
func WithGasLimitMargin(marginPercent uint64) TxBuilderOption {
	return func(b *TxBuilder) {
		b.gasLimitMarginPercent = marginPercent
	}
}

// WithFeeCapMultiplier sets the multiplier for calculating fee cap from gas price
func WithFeeCapMultiplier(multiplier uint64) TxBuilderOption {
	return func(b *TxBuilder) {
		b.feeCapMultiplier = multiplier
	}
}

// TxBuilder helps construct Ethereum transactions
type TxBuilder struct {
	ctx                   context.Context
	chain                 Chain
	supportedTxTypes      []uint8
	forcedTxType          *uint8 // indicates if the tx type was manually set
	gasLimitMarginPercent uint64
	feeCapMultiplier      uint64
}

// NewTxBuilder creates a new transaction builder
func NewTxBuilder(ctx context.Context, chain Chain, opts ...TxBuilderOption) *TxBuilder {
	builder := &TxBuilder{
		chain:                 chain,
		ctx:                   ctx,
		supportedTxTypes:      []uint8{types.LegacyTxType}, // Legacy is always supported
		gasLimitMarginPercent: DefaultGasLimitMarginPercent,
		feeCapMultiplier:      DefaultFeeCapMultiplier,
	}

	// Apply any options provided
	for _, opt := range opts {
		opt(builder)
	}

	// Skip network checks if tx type is forced
	if builder.forcedTxType == nil {
		if builder.chain.SupportsEIP(ctx, 1559) {
			builder.supportedTxTypes = append(builder.supportedTxTypes, types.DynamicFeeTxType)
			builder.supportedTxTypes = append(builder.supportedTxTypes, types.AccessListTxType)
		}
		if builder.chain.SupportsEIP(ctx, 4844) {
			builder.supportedTxTypes = append(builder.supportedTxTypes, types.BlobTxType)
		}
	}

	slog.InfoContext(ctx, "Instantiated TxBuilder",
		"supportedTxTypes", builder.supportedTxTypes,
		"forcedTxType", builder.forcedTxType,
		"gasLimitMargin", builder.gasLimitMarginPercent,
		"feeCapMultiplier", builder.feeCapMultiplier,
	)
	return builder
}

// BuildTx creates a new transaction, using the appropriate type for the network
func (b *TxBuilder) BuildTx(options ...TxOption) (Transaction, error) {
	// Apply options to create TxOpts
	opts := &TxOpts{}
	for _, opt := range options {
		opt(opts)
	}

	// Check for blob transaction requirements if blobs are provided
	if len(opts.blobHashes) > 0 {
		if b.forcedTxType != nil && *b.forcedTxType != types.BlobTxType {
			return nil, fmt.Errorf("blob transactions not supported with forced transaction type %d", *b.forcedTxType)
		}
		if !b.supportsType(types.BlobTxType) {
			return nil, fmt.Errorf("blob transactions not supported by the network")
		}
	}

	// Validate all fields
	if err := opts.Validate(); err != nil {
		return nil, fmt.Errorf("invalid transaction options: %w", err)
	}

	var tx *types.Transaction
	var err error

	// Choose the most advanced supported transaction type
	txType := b.chooseTxType(len(opts.accessList) > 0, len(opts.blobHashes) > 0)
	switch txType {
	case types.BlobTxType:
		if len(opts.blobHashes) > 0 {
			tx, err = b.buildBlobTx(opts)
		} else {
			// If blob tx type is forced but no blobs provided, fall back to dynamic fee tx
			tx, err = b.buildDynamicFeeTx(opts)
		}
	case types.AccessListTxType:
		tx, err = b.buildAccessListTx(opts)
	case types.DynamicFeeTxType:
		tx, err = b.buildDynamicFeeTx(opts)
	default:
		tx, err = b.buildLegacyTx(opts)
	}

	if err != nil {
		return nil, err
	}

	return &EthTx{
		tx:     tx,
		from:   opts.from,
		txType: txType,
	}, nil
}

// supportsType checks if a transaction type is supported
func (b *TxBuilder) supportsType(txType uint8) bool {
	for _, t := range b.supportedTxTypes {
		if t == txType {
			return true
		}
	}
	return false
}

// chooseTxType selects the most advanced supported transaction type
func (b *TxBuilder) chooseTxType(hasAccessList bool, hasBlobs bool) uint8 {
	if b.forcedTxType != nil {
		return *b.forcedTxType
	}

	// Blob transactions are the most advanced, but only use them if we have blobs
	if hasBlobs && b.supportsType(types.BlobTxType) {
		return types.BlobTxType
	}

	// If we have an access list and support access list transactions, use that
	if hasAccessList && b.supportsType(types.AccessListTxType) {
		return types.AccessListTxType
	}

	// Try dynamic fee transactions next
	if b.supportsType(types.DynamicFeeTxType) {
		return types.DynamicFeeTxType
	}

	// Fall back to legacy
	return types.LegacyTxType
}

// getNonce gets the next nonce for the given address
func (b *TxBuilder) getNonce(from common.Address) (uint64, error) {
	nonce, err := b.chain.PendingNonceAt(b.ctx, from)
	if err != nil {
		return 0, fmt.Errorf("failed to get nonce: %w", err)
	}
	return nonce, nil
}

// getGasPrice gets the suggested gas price from the network
func (b *TxBuilder) getGasPrice() (*big.Int, error) {
	gasPrice, err := b.chain.GasPrice(b.ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get gas price: %w", err)
	}
	return gasPrice, nil
}

// calculateGasLimit calculates the gas limit for a transaction, with a configurable safety buffer
func (b *TxBuilder) calculateGasLimit(opts *TxOpts) (uint64, error) {
	if opts.gasLimit != 0 {
		return opts.gasLimit, nil
	}

	estimated, err := b.chain.GasLimit(b.ctx, opts)
	if err != nil {
		return 0, fmt.Errorf("failed to estimate gas: %w", err)
	}
	// Add the configured margin to the estimated gas limit
	return estimated * (100 + b.gasLimitMarginPercent) / 100, nil
}

// buildDynamicFeeTx creates a new EIP-1559 transaction with the given parameters
func (b *TxBuilder) buildDynamicFeeTx(opts *TxOpts) (*types.Transaction, error) {
	nonce, err := b.getNonce(opts.from)
	if err != nil {
		return nil, err
	}

	gasPrice, err := b.getGasPrice()
	if err != nil {
		return nil, err
	}

	chainID := b.chain.ID()

	gasLimit, err := b.calculateGasLimit(opts)
	if err != nil {
		return nil, err
	}

	return types.NewTx(&types.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     nonce,
		GasTipCap: gasPrice,
		GasFeeCap: new(big.Int).Mul(gasPrice, big.NewInt(int64(b.feeCapMultiplier))),
		Gas:       gasLimit,
		To:        opts.to,
		Value:     opts.value,
		Data:      opts.data,
	}), nil
}

// buildLegacyTx creates a new legacy (pre-EIP-1559) transaction
func (b *TxBuilder) buildLegacyTx(opts *TxOpts) (*types.Transaction, error) {
	nonce, err := b.getNonce(opts.from)
	if err != nil {
		return nil, err
	}

	gasPrice, err := b.getGasPrice()
	if err != nil {
		return nil, err
	}

	gasLimit, err := b.calculateGasLimit(opts)
	if err != nil {
		return nil, err
	}

	return types.NewTx(&types.LegacyTx{
		Nonce:    nonce,
		To:       opts.to,
		Value:    opts.value,
		Gas:      gasLimit,
		GasPrice: gasPrice,
		Data:     opts.data,
	}), nil
}

// buildAccessListTx creates a new EIP-2930 transaction with access list
func (b *TxBuilder) buildAccessListTx(opts *TxOpts) (*types.Transaction, error) {
	nonce, err := b.getNonce(opts.from)
	if err != nil {
		return nil, err
	}

	gasPrice, err := b.getGasPrice()
	if err != nil {
		return nil, err
	}

	chainID := b.chain.ID()

	gasLimit, err := b.calculateGasLimit(opts)
	if err != nil {
		return nil, err
	}

	return types.NewTx(&types.AccessListTx{
		ChainID:    chainID,
		Nonce:      nonce,
		GasPrice:   gasPrice,
		Gas:        gasLimit,
		To:         opts.to,
		Value:      opts.value,
		Data:       opts.data,
		AccessList: opts.accessList,
	}), nil
}

// buildBlobTx creates a new EIP-4844 blob transaction
func (b *TxBuilder) buildBlobTx(opts *TxOpts) (*types.Transaction, error) {
	nonce, err := b.getNonce(opts.from)
	if err != nil {
		return nil, err
	}

	gasPrice, err := b.getGasPrice()
	if err != nil {
		return nil, err
	}

	chainID := b.chain.ID()

	gasLimit, err := b.calculateGasLimit(opts)
	if err != nil {
		return nil, err
	}

	// Validate blob transaction requirements
	if opts.to == nil {
		return nil, fmt.Errorf("blob transactions must have a recipient")
	}

	if len(opts.blobHashes) == 0 {
		return nil, fmt.Errorf("blob transactions must have at least one blob hash")
	}

	if len(opts.blobs) != len(opts.commitments) || len(opts.blobs) != len(opts.proofs) {
		return nil, fmt.Errorf("mismatched number of blobs, commitments, and proofs")
	}

	// Convert big.Int values to uint256.Int
	chainIDU256, _ := uint256.FromBig(chainID)
	gasTipCapU256, _ := uint256.FromBig(gasPrice)
	gasFeeCapU256, _ := uint256.FromBig(new(big.Int).Mul(gasPrice, big.NewInt(int64(b.feeCapMultiplier))))
	valueU256, _ := uint256.FromBig(opts.value)
	// For blob transactions, we'll use the same gas price for blob fee cap
	blobFeeCapU256, _ := uint256.FromBig(gasPrice)

	return types.NewTx(&types.BlobTx{
		ChainID:    chainIDU256,
		Nonce:      nonce,
		GasTipCap:  gasTipCapU256,
		GasFeeCap:  gasFeeCapU256,
		Gas:        gasLimit,
		To:         *opts.to,
		Value:      valueU256,
		Data:       opts.data,
		AccessList: opts.accessList,
		BlobFeeCap: blobFeeCapU256,
		BlobHashes: opts.blobHashes,
		Sidecar: &types.BlobTxSidecar{
			Blobs:       opts.blobs,
			Commitments: opts.commitments,
			Proofs:      opts.proofs,
		},
	}), nil
}
