package system

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
	"github.com/stretchr/testify/assert"
)

func TestTxOpts_Validate(t *testing.T) {
	addr := common.HexToAddress("0x1234567890123456789012345678901234567890")
	tests := []struct {
		name    string
		opts    *TxOpts
		wantErr bool
	}{
		{
			name: "valid basic transaction",
			opts: &TxOpts{
				from:  addr,
				to:    &addr,
				value: big.NewInt(0),
			},
			wantErr: false,
		},
		{
			name: "missing from address",
			opts: &TxOpts{
				to:    &addr,
				value: big.NewInt(0),
			},
			wantErr: true,
		},
		{
			name: "missing to address",
			opts: &TxOpts{
				from:  addr,
				value: big.NewInt(0),
			},
			wantErr: true,
		},
		{
			name: "negative value",
			opts: &TxOpts{
				from:  addr,
				to:    &addr,
				value: big.NewInt(-1),
			},
			wantErr: true,
		},
		{
			name: "valid with blobs",
			opts: &TxOpts{
				from:        addr,
				to:          &addr,
				value:       big.NewInt(0),
				blobs:       []kzg4844.Blob{{1}},
				commitments: []kzg4844.Commitment{{2}},
				proofs:      []kzg4844.Proof{{3}},
				blobHashes:  []common.Hash{{4}},
			},
			wantErr: false,
		},
		{
			name: "inconsistent blob fields - missing blobs",
			opts: &TxOpts{
				from:        addr,
				to:          &addr,
				value:       big.NewInt(0),
				commitments: []kzg4844.Commitment{{2}},
				proofs:      []kzg4844.Proof{{3}},
				blobHashes:  []common.Hash{{4}},
			},
			wantErr: true,
		},
		{
			name: "inconsistent blob fields - mismatched lengths",
			opts: &TxOpts{
				from:        addr,
				to:          &addr,
				value:       big.NewInt(0),
				blobs:       []kzg4844.Blob{{1}},
				commitments: []kzg4844.Commitment{{2}, {3}}, // Extra commitment
				proofs:      []kzg4844.Proof{{3}},
				blobHashes:  []common.Hash{{4}},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.opts.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTxOpts_Getters(t *testing.T) {
	addr := common.HexToAddress("0x1234567890123456789012345678901234567890")
	value := big.NewInt(123)
	data := []byte{1, 2, 3}

	opts := &TxOpts{
		from:  addr,
		to:    &addr,
		value: value,
		data:  data,
	}

	assert.Equal(t, addr, opts.From())
	assert.Equal(t, &addr, opts.To())
	assert.Equal(t, value, opts.Value())
	assert.Equal(t, data, opts.Data())
}

func TestEthTx_Methods(t *testing.T) {
	addr := common.HexToAddress("0x1234567890123456789012345678901234567890")
	value := big.NewInt(123)
	data := []byte{1, 2, 3}

	// Create a legacy transaction for testing
	tx := types.NewTransaction(
		0,             // nonce
		addr,          // to
		value,         // value
		21000,         // gas limit
		big.NewInt(1), // gas price
		data,          // data
	)

	ethTx := &EthTx{
		tx:     tx,
		from:   addr,
		txType: uint8(types.LegacyTxType),
	}

	assert.Equal(t, tx.Hash(), ethTx.Hash())
	assert.Equal(t, addr, ethTx.From())
	assert.Equal(t, &addr, ethTx.To())
	assert.Equal(t, value, ethTx.Value())
	assert.Equal(t, data, ethTx.Data())
	assert.Equal(t, uint8(types.LegacyTxType), ethTx.Type())
	assert.Equal(t, tx, ethTx.Raw())
}

func TestTxOptions(t *testing.T) {
	addr := common.HexToAddress("0x1234567890123456789012345678901234567890")
	value := big.NewInt(123)
	data := []byte{1, 2, 3}
	gasLimit := uint64(21000)
	accessList := types.AccessList{{
		Address:     addr,
		StorageKeys: []common.Hash{{1}},
	}}
	blobs := []kzg4844.Blob{{1}}
	commitments := []kzg4844.Commitment{{2}}
	proofs := []kzg4844.Proof{{3}}
	blobHashes := []common.Hash{{4}}

	opts := &TxOpts{}

	// Apply all options
	WithFrom(addr)(opts)
	WithTo(addr)(opts)
	WithValue(value)(opts)
	WithData(data)(opts)
	WithGasLimit(gasLimit)(opts)
	WithAccessList(accessList)(opts)
	WithBlobs(blobs)(opts)
	WithBlobCommitments(commitments)(opts)
	WithBlobProofs(proofs)(opts)
	WithBlobHashes(blobHashes)(opts)

	// Verify all fields were set correctly
	assert.Equal(t, addr, opts.from)
	assert.Equal(t, &addr, opts.to)
	assert.Equal(t, value, opts.value)
	assert.Equal(t, data, opts.data)
	assert.Equal(t, gasLimit, opts.gasLimit)
	assert.Equal(t, accessList, opts.accessList)
	assert.Equal(t, blobs, opts.blobs)
	assert.Equal(t, commitments, opts.commitments)
	assert.Equal(t, proofs, opts.proofs)
	assert.Equal(t, blobHashes, opts.blobHashes)
}
