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
	GasPrice             *hexutil.U256   `json:"gasPrice"`
	MaxFeePerGas         *hexutil.U256   `json:"maxFeePerGas"`
	MaxPriorityFeePerGas *hexutil.U256   `json:"maxPriorityFeePerGas"`
	Value                *hexutil.U256   `json:"value"`
	Nonce                *hexutil.Uint64 `json:"nonce"`

	// We accept "data" and "input" for backwards-compatibility reasons.
	// "input" is the newer name and should be preferred by clients.
	// Issue detail: https://github.com/ethereum/go-ethereum/issues/15628
	Data  *hexutil.Bytes `json:"data"`
	Input *hexutil.Bytes `json:"input"`

	AccessList *types.AccessList `json:"accessList,omitempty"`
	ChainID    *hexutil.U256     `json:"chainId,omitempty"`

	// Custom extension for EIP-4844 support
	BlobVersionedHashes []common.Hash `json:"blobVersionedHashes,omitempty"`
	BlobFeeCap          *hexutil.U256 `json:"maxFeePerBlobGas,omitempty"`

	// Custom extension for EIP-7702 support
	AuthList []types.SetCodeAuthorization `json:"authorizationList,omitempty"`
}

// NewTransactionArgsFromTransaction creates a TransactionArgs struct from an EIP-1559, EIP-4844, or EIP-7702 transaction
func NewTransactionArgsFromTransaction(chainId *big.Int, from *common.Address, tx *types.Transaction) *TransactionArgs {
	data := hexutil.Bytes(tx.Data())
	nonce := hexutil.Uint64(tx.Nonce())
	gas := hexutil.Uint64(tx.Gas())
	accesses := tx.AccessList()

	return &TransactionArgs{
		From:                 from,
		Input:                &data,
		Nonce:                &nonce,
		Value:                (*hexutil.U256)(uint256.MustFromBig(tx.Value())),
		Gas:                  &gas,
		To:                   tx.To(),
		ChainID:              (*hexutil.U256)(uint256.MustFromBig(chainId)),
		MaxFeePerGas:         (*hexutil.U256)(uint256.MustFromBig(tx.GasFeeCap())),
		MaxPriorityFeePerGas: (*hexutil.U256)(uint256.MustFromBig(tx.GasTipCap())),
		AccessList:           &accesses,
		BlobVersionedHashes:  tx.BlobHashes(),
		BlobFeeCap:           (*hexutil.U256)(uint256.MustFromBig(tx.BlobGasFeeCap())),
		AuthList:             tx.SetCodeAuthorizations(),
	}
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
	if (*uint256.Int)(args.MaxFeePerGas).Cmp((*uint256.Int)(args.MaxPriorityFeePerGas)) < 0 {
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
		args.Value = new(hexutil.U256)
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
		data = &types.SetCodeTx{
			ChainID:    (*uint256.Int)(args.ChainID),
			Nonce:      uint64(*args.Nonce),
			GasTipCap:  (*uint256.Int)(args.MaxPriorityFeePerGas),
			GasFeeCap:  (*uint256.Int)(args.MaxFeePerGas),
			Gas:        uint64(*args.Gas),
			To:         *args.To,
			Value:      (*uint256.Int)(args.Value),
			Data:       args.data(),
			AccessList: al,
			AuthList:   args.AuthList,
		}
	} else if len(args.BlobVersionedHashes) > 0 {
		data = &types.BlobTx{
			ChainID:    (*uint256.Int)(args.ChainID),
			Nonce:      uint64(*args.Nonce),
			GasTipCap:  (*uint256.Int)(args.MaxPriorityFeePerGas),
			GasFeeCap:  (*uint256.Int)(args.MaxFeePerGas),
			Gas:        uint64(*args.Gas),
			To:         *args.To,
			Value:      (*uint256.Int)(args.Value),
			Data:       args.data(),
			AccessList: al,
			BlobFeeCap: (*uint256.Int)(args.BlobFeeCap),
			BlobHashes: args.BlobVersionedHashes,
		}
	} else {
		data = &types.DynamicFeeTx{
			ChainID:    (*uint256.Int)(args.ChainID).ToBig(),
			Nonce:      uint64(*args.Nonce),
			GasTipCap:  (*uint256.Int)(args.MaxPriorityFeePerGas).ToBig(),
			GasFeeCap:  (*uint256.Int)(args.MaxFeePerGas).ToBig(),
			Gas:        uint64(*args.Gas),
			To:         args.To,
			Value:      (*uint256.Int)(args.Value).ToBig(),
			Data:       args.data(),
			AccessList: al,
		}
	}
	return data, nil
}
