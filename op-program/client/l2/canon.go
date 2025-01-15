package l2

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type CanonicalBlockHeaderOracle struct {
	head                 *types.Header
	hashByNum            map[uint64]common.Hash
	earliestIndexedBlock *types.Header
	oracle               Oracle
	chainID              uint64
}

func NewCanonicalBlockHeaderOracle(head *types.Header, oracle Oracle, chainID uint64) *CanonicalBlockHeaderOracle {
	return &CanonicalBlockHeaderOracle{
		head: head,
		hashByNum: map[uint64]common.Hash{
			head.Number.Uint64(): head.Hash(),
		},
		earliestIndexedBlock: head,
		oracle:               oracle,
		chainID:              chainID,
	}
}

// GetHeaderByNumber walks back from the current head to the requested block number
func (o *CanonicalBlockHeaderOracle) GetHeaderByNumber(n uint64) *types.Header {
	if o.head.Number.Uint64() < n {
		return nil
	}

	var h *types.Header
	if o.earliestIndexedBlock.Number.Uint64() > n {
		hash, ok := o.hashByNum[n]
		if ok {
			return o.oracle.BlockByHash(hash, o.chainID).Header()
		}
		h = o.head
	} else {
		h = o.oracle.BlockByHash(o.hashByNum[n], o.chainID).Header()
	}

	for h.Number.Uint64() > n {
		h = o.oracle.BlockByHash(h.ParentHash, o.chainID).Header()
		o.hashByNum[h.Number.Uint64()] = h.Hash()
	}
	o.earliestIndexedBlock = h
	return h
}

func (o *CanonicalBlockHeaderOracle) SetCanonical(head *types.Header) common.Hash {
	oldHead := o.head
	o.head = head

	// Remove canonical hashes after the new header
	for n := head.Number.Uint64() + 1; n <= oldHead.Number.Uint64(); n++ {
		delete(o.hashByNum, n)
	}

	// Add new canonical blocks to the block by number cache
	// Since the original head is added to the block number cache and acts as the finalized block,
	// at some point we must reach the existing canonical chain and can stop updating.
	h := o.head
	for {
		newHash := h.Hash()
		prevHash, ok := o.hashByNum[h.Number.Uint64()]
		if ok && prevHash == newHash {
			// Connected with the existing canonical chain so stop updating
			break
		}
		o.hashByNum[h.Number.Uint64()] = newHash
		h = o.oracle.BlockByHash(h.ParentHash, o.chainID).Header()
	}
	o.earliestIndexedBlock = h
	return head.Hash()
}
