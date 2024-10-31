package eth

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/misc/eip4844"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
)

type BlockInfo interface {
	Hash() common.Hash
	ParentHash() common.Hash
	Coinbase() common.Address
	Root() common.Hash // state-root
	NumberU64() uint64
	Time() uint64
	// MixDigest field, reused for randomness after The Merge (Bellatrix hardfork)
	MixDigest() common.Hash
	BaseFee() *big.Int
	// BlobBaseFee returns the result of computing the blob fee from excessDataGas, or nil if the
	// block isn't a Dencun (4844 capable) block
	BlobBaseFee() *big.Int
	ReceiptHash() common.Hash
	GasUsed() uint64
	GasLimit() uint64
	ParentBeaconRoot() *common.Hash // Dencun extension
	WithdrawalsRoot() *common.Hash  // Isthmus extension

	// HeaderRLP returns the RLP of the block header as per consensus rules
	// Returns an error if the header RLP could not be written
	HeaderRLP() ([]byte, error)
}

func InfoToL1BlockRef(info BlockInfo) L1BlockRef {
	return L1BlockRef{
		Hash:       info.Hash(),
		Number:     info.NumberU64(),
		ParentHash: info.ParentHash(),
		Time:       info.Time(),
	}
}

type NumberAndHash interface {
	Hash() common.Hash
	NumberU64() uint64
}

func ToBlockID(b NumberAndHash) BlockID {
	return BlockID{
		Hash:   b.Hash(),
		Number: b.NumberU64(),
	}
}

// blockInfo is a conversion type of types.Block turning it into a BlockInfo
type blockInfo struct{ *types.Block }

func (b blockInfo) BlobBaseFee() *big.Int {
	ebg := b.ExcessBlobGas()
	if ebg == nil {
		return nil
	}
	return eip4844.CalcBlobFee(*ebg)
}

func (b blockInfo) HeaderRLP() ([]byte, error) {
	return rlp.EncodeToBytes(b.Header())
}

func (b blockInfo) ParentBeaconRoot() *common.Hash {
	return b.Block.BeaconRoot()
}

func (b blockInfo) WithdrawalsRoot() *common.Hash {
	return b.Header().WithdrawalsHash
}

func BlockToInfo(b *types.Block) BlockInfo {
	return blockInfo{b}
}

var _ BlockInfo = (*blockInfo)(nil)

// headerBlockInfo is a conversion type of types.Header turning it into a
// BlockInfo.
type headerBlockInfo struct{ *types.Header }

func (h headerBlockInfo) ParentHash() common.Hash {
	return h.Header.ParentHash
}

func (h headerBlockInfo) Coinbase() common.Address {
	return h.Header.Coinbase
}

func (h headerBlockInfo) Root() common.Hash {
	return h.Header.Root
}

func (h headerBlockInfo) NumberU64() uint64 {
	return h.Header.Number.Uint64()
}

func (h headerBlockInfo) Time() uint64 {
	return h.Header.Time
}

func (h headerBlockInfo) MixDigest() common.Hash {
	return h.Header.MixDigest
}

func (h headerBlockInfo) BaseFee() *big.Int {
	return h.Header.BaseFee
}

func (h headerBlockInfo) BlobBaseFee() *big.Int {
	if h.ExcessBlobGas == nil {
		return nil
	}
	return eip4844.CalcBlobFee(*h.ExcessBlobGas)
}

func (h headerBlockInfo) ReceiptHash() common.Hash {
	return h.Header.ReceiptHash
}

func (h headerBlockInfo) GasUsed() uint64 {
	return h.Header.GasUsed
}

func (h headerBlockInfo) GasLimit() uint64 {
	return h.Header.GasLimit
}

func (h headerBlockInfo) ParentBeaconRoot() *common.Hash {
	return h.Header.ParentBeaconRoot
}

func (h headerBlockInfo) WithdrawalsRoot() *common.Hash {
	return h.Header.WithdrawalsHash
}

func (h headerBlockInfo) HeaderRLP() ([]byte, error) {
	return rlp.EncodeToBytes(h.Header)
}

// HeaderBlockInfo returns h as a BlockInfo implementation.
func HeaderBlockInfo(h *types.Header) BlockInfo {
	return headerBlockInfo{h}
}
