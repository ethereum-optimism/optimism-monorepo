package signer

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"

	"github.com/holiman/uint256"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
)

// TransactionArgs represents the arguments to construct a new transaction
// or a message call.
// Geth has an internal version of this, but this is not exported, and only supported in v1.13.11 and forward.
// This signing API format is based on the legacy personal-account signing RPC of ethereum.
type TransactionArgs struct {
	From                 *common.Address `json:"from"`
	To                   *common.Address `json:"to"`
	Gas                  *hexutil.Uint64 `json:"gas"`
	GasPrice             *hexutil.Big    `json:"gasPrice"`
	MaxFeePerGas         *hexutil.Big    `json:"maxFeePerGas"`
	MaxPriorityFeePerGas *hexutil.Big    `json:"maxPriorityFeePerGas"`
	Value                *hexutil.Big    `json:"value"`
	Nonce                *hexutil.Uint64 `json:"nonce"`

	// We accept "data" and "input" for backwards-compatibility reasons.
	// "input" is the newer name and should be preferred by clients.
	// Issue detail: https://github.com/ethereum/go-ethereum/issues/15628
	Data  *hexutil.Bytes `json:"data"`
	Input *hexutil.Bytes `json:"input"`

	AccessList *types.AccessList `json:"accessList,omitempty"`
	ChainID    *hexutil.Big      `json:"chainId,omitempty"`

	// Custom extension for EIP-4844 support
	BlobVersionedHashes []common.Hash `json:"blobVersionedHashes,omitempty"`
	BlobFeeCap          *hexutil.Big  `json:"maxFeePerBlobGas,omitempty"`

	// Custom extension for EIP-7702 support
	AuthList []types.SetCodeAuthorization `json:"authList,omitempty"`
}

// NewTransactionArgsFromTransaction creates a TransactionArgs struct from an EIP-1559 or EIP-4844 transaction
func NewTransactionArgsFromTransaction(chainId *big.Int, from *common.Address, tx *types.Transaction) *TransactionArgs {
	data := hexutil.Bytes(tx.Data())
	nonce := hexutil.Uint64(tx.Nonce())
	gas := hexutil.Uint64(tx.Gas())
	accesses := tx.AccessList()
	args := &TransactionArgs{
		From:                 from,
		Input:                &data,
		Nonce:                &nonce,
		Value:                (*hexutil.Big)(tx.Value()),
		Gas:                  &gas,
		To:                   tx.To(),
		ChainID:              (*hexutil.Big)(chainId),
		MaxFeePerGas:         (*hexutil.Big)(tx.GasFeeCap()),
		MaxPriorityFeePerGas: (*hexutil.Big)(tx.GasTipCap()),
		AccessList:           &accesses,
		BlobVersionedHashes:  tx.BlobHashes(),
		BlobFeeCap:           (*hexutil.Big)(tx.BlobGasFeeCap()),
		AuthList:             tx.SetCodeAuthorizations(),
	}

	return args
}

// data retrieves the transaction calldata. Input field is preferred.
func (args *TransactionArgs) data() []byte {
	if args.Input != nil {
		return *args.Input
	}
	if args.Data != nil {
		return *args.Data
	}
	return nil
}

func (args *TransactionArgs) Check() error {
	if args.Gas == nil {
		return errors.New("gas not specified")
	}
	if args.GasPrice != nil {
		return errors.New("only accepts maxFeePerGas/maxPriorityFeePerGas params")
	}
	if args.MaxFeePerGas == nil || args.MaxPriorityFeePerGas == nil {
		return errors.New("missing maxFeePerGas or maxPriorityFeePerGas")
	}
	// Both EIP-1559 fee parameters are now set; sanity check them.
	if args.MaxFeePerGas.ToInt().Cmp(args.MaxPriorityFeePerGas.ToInt()) < 0 {
		return fmt.Errorf("maxFeePerGas (%v) < maxPriorityFeePerGas (%v)", args.MaxFeePerGas, args.MaxPriorityFeePerGas)
	}
	if args.Nonce == nil {
		return errors.New("nonce not specified")
	}
	if args.Data != nil && args.Input != nil && !bytes.Equal(*args.Data, *args.Input) {
		return errors.New(`both "data" and "input" are set and not equal. Please use "input" to pass transaction call data`)
	}
	if args.To == nil && len(args.data()) == 0 {
		return errors.New("contract creation without any data provided")
	}
	if args.ChainID == nil {
		return errors.New("chain id not specified")
	}
	if args.Value == nil {
		args.Value = new(hexutil.Big)
	}
	if args.AccessList == nil {
		args.AccessList = &types.AccessList{}
	}
	if args.BlobVersionedHashes != nil {
		if len(args.BlobVersionedHashes) == 0 {
			return errors.New("non-null blob versioned hashes should not be empty")
		}
		if args.BlobFeeCap == nil {
			return errors.New("when including blobs a blob-fee-cap is required")
		}
	} else {
		if args.BlobFeeCap != nil {
			return errors.New("unexpected blob-fee-cap, transaction does not include blobs")
		}
	}
	if args.AuthList != nil {
		if len(args.AuthList) == 0 {
			return errors.New("non-null auth list should not be empty")
		}
	}
	return nil
}

// ToTransactionData converts the arguments to transaction content-data. Warning: this excludes blob data.
func (args *TransactionArgs) ToTransactionData() (types.TxData, error) {
	var data types.TxData
	al := types.AccessList{}
	if args.AccessList != nil {
		al = *args.AccessList
	}

	if args.AuthList != nil {
		fields, err := args.convertToUint256()
		if err != nil {
			return nil, err
		}
		data = &types.SetCodeTx{
			ChainID:    fields.chainID,
			Nonce:      uint64(*args.Nonce),
			GasTipCap:  fields.maxPriorityFeePerGas,
			GasFeeCap:  fields.maxFeePerGas,
			Gas:        uint64(*args.Gas),
			To:         *args.To,
			Value:      fields.value,
			Data:       args.data(),
			AccessList: al,
			AuthList:   args.AuthList,
		}
	} else if len(args.BlobVersionedHashes) > 0 {
		fields, err := args.convertToUint256()
		if err != nil {
			return nil, err
		}
		data = &types.BlobTx{
			ChainID:    fields.chainID,
			Nonce:      uint64(*args.Nonce),
			GasTipCap:  fields.maxPriorityFeePerGas,
			GasFeeCap:  fields.maxFeePerGas,
			Gas:        uint64(*args.Gas),
			To:         *args.To,
			Value:      fields.value,
			Data:       args.data(),
			AccessList: al,
			BlobFeeCap: fields.blobFeeCap,
			BlobHashes: args.BlobVersionedHashes,
		}
	} else {
		data = &types.DynamicFeeTx{
			ChainID:    (*big.Int)(args.ChainID),
			Nonce:      uint64(*args.Nonce),
			GasTipCap:  (*big.Int)(args.MaxPriorityFeePerGas),
			GasFeeCap:  (*big.Int)(args.MaxFeePerGas),
			Gas:        uint64(*args.Gas),
			To:         args.To,
			Value:      (*big.Int)(args.Value),
			Data:       args.data(),
			AccessList: al,
		}
	}
	return data, nil
}

type uint256Fields struct {
	chainID              *uint256.Int
	maxFeePerGas         *uint256.Int
	maxPriorityFeePerGas *uint256.Int
	value                *uint256.Int
	blobFeeCap           *uint256.Int
}

func (args *TransactionArgs) convertToUint256() (*uint256Fields, error) {
	chainID, overflow := uint256.FromBig((*big.Int)(args.ChainID))
	if overflow {
		return nil, fmt.Errorf("chainID %s too large for tx", args.ChainID)
	}
	maxFeePerGas, overflow := uint256.FromBig((*big.Int)(args.MaxFeePerGas))
	if overflow {
		return nil, fmt.Errorf("maxFeePerGas %s too large for tx", args.MaxFeePerGas)
	}
	maxPriorityFeePerGas, overflow := uint256.FromBig((*big.Int)(args.MaxPriorityFeePerGas))
	if overflow {
		return nil, fmt.Errorf("maxPriorityFeePerGas %s too large for tx", args.MaxPriorityFeePerGas)
	}
	value, overflow := uint256.FromBig((*big.Int)(args.Value))
	if overflow {
		return nil, fmt.Errorf("value %s too large for tx", args.Value)
	}
	blobFeeCap, overflow := uint256.FromBig((*big.Int)(args.BlobFeeCap))
	if overflow {
		return nil, fmt.Errorf("blobFeeCap %s too large for blob tx", args.BlobFeeCap)
	}
	return &uint256Fields{
		chainID:              chainID,
		maxFeePerGas:         maxFeePerGas,
		maxPriorityFeePerGas: maxPriorityFeePerGas,
		value:                value,
		blobFeeCap:           blobFeeCap,
	}, nil
}
