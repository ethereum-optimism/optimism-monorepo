package txplan

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/plan"
)

type PlannedTx struct {
	// Block that we schedule against
	AgainstBlock plan.Lazy[*types.Header]
	Unsigned     plan.Lazy[types.TxData]
	Signed       plan.Lazy[*types.Transaction]
	Included     plan.Lazy[*types.Receipt]
	Success      plan.Lazy[struct{}]

	Signer plan.Lazy[types.Signer]
	Priv   plan.Lazy[*ecdsa.PrivateKey]

	Sender plan.Lazy[common.Address]

	// How much more gas to use as limit than estimated
	GasRatio plan.Lazy[float64]

	Type       plan.Lazy[uint8]
	Data       plan.Lazy[hexutil.Bytes]
	ChainID    plan.Lazy[eth.ChainID]
	Nonce      plan.Lazy[uint64]
	GasTipCap  plan.Lazy[*big.Int]
	GasFeeCap  plan.Lazy[*big.Int]
	Gas        plan.Lazy[uint64]
	To         plan.Lazy[*common.Address]
	Value      plan.Lazy[*big.Int]
	AccessList plan.Lazy[types.AccessList] // resolves to nil if not an attribute
	//AuthList   plan.Lazy[struct{}]         // resolves to nil if not a 7702 tx
}

func (ptx *PlannedTx) String() string {
	// success case should capture all contents
	return "PlannedTx:\n" + ptx.Success.String()
}

type Option func(tx *PlannedTx)

func NewPlannedTx(opts ...Option) *PlannedTx {
	tx := &PlannedTx{}
	tx.Defaults()
	for _, opt := range opts {
		opt(tx)
	}
	return tx
}

func WithValue(val *big.Int) Option {
	return func(tx *PlannedTx) {
		tx.Value.Set(val)
	}
}

func WithAccessList(al types.AccessList) Option {
	return func(tx *PlannedTx) {
		tx.AccessList.Set(al)
	}
}

func WithPrivateKey(priv *ecdsa.PrivateKey) Option {
	return func(tx *PlannedTx) {
		tx.Priv.Set(priv)
	}
}

type Estimator interface {
	EstimateGas(ctx context.Context, msg ethereum.CallMsg) (uint64, error)
}

func WithEstimator(cl Estimator, invalidateOnNewBlock bool) Option {
	return func(tx *PlannedTx) {
		tx.Gas.DependOn(
			&tx.Sender,
			&tx.To,
			&tx.GasFeeCap,
			&tx.GasTipCap,
			&tx.Value,
			&tx.Data,
			&tx.AccessList,
			&tx.GasRatio,
		)
		if invalidateOnNewBlock {
			tx.Gas.DependOn(&tx.AgainstBlock)
		}
		tx.Gas.Fn(func(ctx context.Context) (uint64, error) {
			msg := ethereum.CallMsg{
				From:       tx.Sender.Value(),
				To:         tx.To.Value(),
				Gas:        0, // infinite gas, will be estimated
				GasPrice:   nil,
				GasFeeCap:  tx.GasFeeCap.Value(),
				GasTipCap:  tx.GasTipCap.Value(),
				Value:      tx.Value.Value(),
				Data:       tx.Data.Value(),
				AccessList: tx.AccessList.Value(),
			}
			gas, err := cl.EstimateGas(ctx, msg)
			if err != nil {
				return 0, err
			}
			ratio := tx.GasRatio.Value()
			gas = uint64(float64(gas) * ratio)
			return gas, nil
		})
	}
}

type ReceiptGetter interface {
	TransactionReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error)
}

// WithAssumedInclusion assumes inclusion at the time of evaluation,
// and simply looks up the tx without blocking on inclusion.
func WithAssumedInclusion(cl ReceiptGetter) Option {
	return func(tx *PlannedTx) {
		tx.Included.DependOn(&tx.Signed)
		tx.Included.Fn(func(ctx context.Context) (*types.Receipt, error) {
			return cl.TransactionReceipt(ctx, tx.Signed.Value().Hash())
		})
	}
}

type PendingNonceAt interface {
	PendingNonceAt(ctx context.Context, account common.Address) (uint64, error)
}

// WithPendingNonce automatically
func WithPendingNonce(cl PendingNonceAt) Option {
	return func(tx *PlannedTx) {
		tx.Nonce.DependOn(&tx.AgainstBlock, &tx.Sender)
		tx.Nonce.Fn(func(ctx context.Context) (uint64, error) {
			return cl.PendingNonceAt(ctx, tx.Sender.Value())
		})
	}
}

func (tx *PlannedTx) Defaults() {
	tx.Type.Set(types.DynamicFeeTxType)
	tx.To.Set(nil)
	tx.Data.Set([]byte{})
	tx.ChainID.Set(eth.ChainIDFromUInt64(1))
	tx.GasRatio.Set(1.0)
	tx.GasTipCap.Set(big.NewInt(1e9)) // 1 gwei
	tx.Gas.Set(params.TxGas)
	tx.Value.Set(big.NewInt(0))
	tx.Nonce.Set(0)
	tx.AccessList.Set(types.AccessList{})

	// Bump the fee-cap to be at least as high as the tip-cap,
	// and as high as the basefee.
	tx.GasFeeCap.DependOn(&tx.GasTipCap, &tx.AgainstBlock)
	tx.GasFeeCap.Fn(func(ctx context.Context) (*big.Int, error) {
		tip := tx.GasTipCap.Value()
		basefee := tx.AgainstBlock.Value().BaseFee
		feeCap := big.NewInt(0)
		feeCap = feeCap.Add(tip, basefee)
		return feeCap, nil
	})

	// Automatically determine tx-signer from chainID
	tx.Signer.DependOn(&tx.ChainID)
	tx.Signer.Fn(func(ctx context.Context) (types.Signer, error) {
		chainID := tx.ChainID.Value()
		return types.LatestSignerForChainID(chainID.ToBig()), nil
	})

	// Automatically determine sender from private key
	tx.Sender.DependOn(&tx.Priv)
	tx.Sender.Fn(func(ctx context.Context) (common.Address, error) {
		return crypto.PubkeyToAddress(tx.Priv.Value().PublicKey), nil
	})

	// Automatically build tx from the individual attributes
	tx.Unsigned.DependOn(
		&tx.Sender,
		&tx.Type,
		&tx.Data,
		&tx.ChainID,
		&tx.Nonce,
		&tx.GasTipCap,
		&tx.GasFeeCap,
		&tx.Gas,
		&tx.To,
		&tx.Value,
		&tx.AccessList,
		//&tx.AuthList,
	)
	tx.Unsigned.Fn(func(ctx context.Context) (types.TxData, error) {
		chainID := tx.ChainID.Value()
		switch tx.Type.Value() {
		case types.LegacyTxType:
			return &types.LegacyTx{
				Nonce:    tx.Nonce.Value(),
				GasPrice: tx.GasFeeCap.Value(),
				Gas:      tx.Gas.Value(),
				To:       tx.To.Value(),
				Value:    tx.Value.Value(),
				Data:     tx.Data.Value(),
				V:        nil,
				R:        nil,
				S:        nil,
			}, nil
		case types.AccessListTxType:
			return &types.AccessListTx{
				ChainID:    chainID.ToBig(),
				Nonce:      tx.Nonce.Value(),
				GasPrice:   tx.GasFeeCap.Value(),
				Gas:        tx.Gas.Value(),
				To:         tx.To.Value(),
				Value:      tx.Value.Value(),
				Data:       tx.Data.Value(),
				AccessList: tx.AccessList.Value(),
				V:          nil,
				R:          nil,
				S:          nil,
			}, nil
		case types.DynamicFeeTxType:
			return &types.DynamicFeeTx{
				ChainID:    chainID.ToBig(),
				Nonce:      tx.Nonce.Value(),
				GasTipCap:  tx.GasTipCap.Value(),
				GasFeeCap:  tx.GasFeeCap.Value(),
				Gas:        tx.Gas.Value(),
				To:         tx.To.Value(),
				Value:      tx.Value.Value(),
				Data:       tx.Data.Value(),
				AccessList: tx.AccessList.Value(),
				V:          nil,
				R:          nil,
				S:          nil,
			}, nil
		case types.BlobTxType:
			return nil, errors.New("blob tx not supported")
		case types.DepositTxType:
			return nil, errors.New("deposit tx not supported")
		default:
			return nil, fmt.Errorf("unrecognized tx type: %d", tx.Type.Value())
		}
	})
	// Sign with the available key
	tx.Signed.DependOn(&tx.Priv, &tx.Signer, &tx.Unsigned)
	tx.Signed.Fn(func(ctx context.Context) (*types.Transaction, error) {
		innerTx := tx.Unsigned.Value()
		signer := tx.Signer.Value()
		prv := tx.Priv.Value()
		return types.SignNewTx(prv, signer, innerTx)
	})

	tx.Success.DependOn(&tx.Included)
	tx.Success.Fn(func(ctx context.Context) (struct{}, error) {
		rec, err := tx.Included.Get()
		if err != nil {
			return struct{}{}, err
		}
		if rec.Status == types.ReceiptStatusSuccessful {
			return struct{}{}, nil
		} else {
			return struct{}{}, errors.New("tx failed")
		}
	})
}
