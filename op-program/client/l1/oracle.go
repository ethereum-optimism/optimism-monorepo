package l1

import (
	"encoding/binary"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"

	preimage "github.com/ethereum-optimism/optimism/op-preimage"
	"github.com/ethereum-optimism/optimism/op-program/client/mpt"
	"github.com/ethereum-optimism/optimism/op-service/eth"
)

type Oracle interface {
	// HeaderByBlockHash retrieves the block header with the given hash.
	HeaderByBlockHash(blockHash common.Hash) eth.BlockInfo

	// TransactionsByBlockHash retrieves the transactions from the block with the given hash.
	TransactionsByBlockHash(blockHash common.Hash) (eth.BlockInfo, types.Transactions)

	// ReceiptsByBlockHash retrieves the receipts from the block with the given hash.
	ReceiptsByBlockHash(blockHash common.Hash) (eth.BlockInfo, types.Receipts)

	// GetBlob retrieves the blob with the given hash.
	GetBlob(ref eth.L1BlockRef, blobHash eth.IndexedBlobHash) *eth.Blob

	// Precompile retrieves the result and success indicator of a precompile call for the given input.
	Precompile(precompileAddress common.Address, input []byte, requiredGas uint64) ([]byte, bool)
}

// PreimageOracle implements Oracle using by interfacing with the pure preimage.Oracle
// to fetch pre-images to decode into the requested data.
type PreimageOracle struct {
	oracle preimage.Oracle
	hint   preimage.Hinter
}

var _ Oracle = (*PreimageOracle)(nil)

func NewPreimageOracle(raw preimage.Oracle, hint preimage.Hinter) *PreimageOracle {
	return &PreimageOracle{
		oracle: raw,
		hint:   hint,
	}
}

func (p *PreimageOracle) headerByBlockHash(blockHash common.Hash) *types.Header {
	p.hint.Hint(BlockHeaderHint(blockHash))
	headerRlp := p.oracle.Get(preimage.Keccak256Key(blockHash))
	var header types.Header
	if err := rlp.DecodeBytes(headerRlp, &header); err != nil {
		panic(fmt.Errorf("invalid block header %s: %w", blockHash, err))
	}
	return &header
}

func (p *PreimageOracle) HeaderByBlockHash(blockHash common.Hash) eth.BlockInfo {
	return eth.HeaderBlockInfoTrusted(blockHash, p.headerByBlockHash(blockHash))
}

func (p *PreimageOracle) TransactionsByBlockHash(blockHash common.Hash) (eth.BlockInfo, types.Transactions) {
	header := p.headerByBlockHash(blockHash)
	p.hint.Hint(TransactionsHint(blockHash))

	opaqueTxs := mpt.ReadTrie(header.TxHash, func(key common.Hash) []byte {
		return p.oracle.Get(preimage.Keccak256Key(key))
	})

	txs, err := eth.DecodeTransactions(opaqueTxs)
	if err != nil {
		panic(fmt.Errorf("failed to decode list of txs: %w", err))
	}

	return eth.HeaderBlockInfoTrusted(blockHash, header), txs
}

func (p *PreimageOracle) ReceiptsByBlockHash(blockHash common.Hash) (eth.BlockInfo, types.Receipts) {
	info, txs := p.TransactionsByBlockHash(blockHash)

	p.hint.Hint(ReceiptsHint(blockHash))

	opaqueReceipts := mpt.ReadTrie(info.ReceiptHash(), func(key common.Hash) []byte {
		return p.oracle.Get(preimage.Keccak256Key(key))
	})

	txHashes := eth.TransactionsToHashes(txs)
	receipts, err := eth.DecodeRawReceipts(eth.ToBlockID(info), opaqueReceipts, txHashes)
	if err != nil {
		panic(fmt.Errorf("bad receipts data for block %s: %w", blockHash, err))
	}

	return info, receipts
}

func (p *PreimageOracle) GetBlob(ref eth.L1BlockRef, blobHash eth.IndexedBlobHash) *eth.Blob {
	// Send a hint for the blob commitment & blob field elements.
	blobReqMeta := make([]byte, 16)
	binary.BigEndian.PutUint64(blobReqMeta[0:8], blobHash.Index)
	binary.BigEndian.PutUint64(blobReqMeta[8:16], ref.Time)
	p.hint.Hint(BlobHint(append(blobHash.Hash[:], blobReqMeta...)))

	commitment := p.oracle.Get(preimage.Sha256Key(blobHash.Hash))

	// Reconstruct the full blob from the 4096 field elements.
	blob := eth.Blob{}
	fieldElemKey := make([]byte, 80)
	copy(fieldElemKey[:48], commitment)
	for i := 0; i < params.BlobTxFieldElementsPerBlob; i++ {
		binary.BigEndian.PutUint64(fieldElemKey[72:], uint64(i))
		fieldElement := p.oracle.Get(preimage.BlobKey(crypto.Keccak256(fieldElemKey)))

		copy(blob[i<<5:(i+1)<<5], fieldElement[:])
	}

	return &blob
}

func (p *PreimageOracle) Precompile(address common.Address, input []byte, requiredGas uint64) ([]byte, bool) {
	hintBytes := append(address.Bytes(), binary.BigEndian.AppendUint64(nil, requiredGas)...)
	hintBytes = append(hintBytes, input...)
	p.hint.Hint(PrecompileHintV2(hintBytes))
	key := preimage.PrecompileKey(crypto.Keccak256Hash(hintBytes))
	result := p.oracle.Get(key)
	if len(result) == 0 { // must contain at least the status code
		panic(fmt.Errorf("unexpected precompile oracle behavior, got result: %x", result))
	}
	return result[1:], result[0] == 1
}
