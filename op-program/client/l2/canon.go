package l2

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type BlockByHashFn func(hash common.Hash) *types.Block

type CanonicalBlockHeaderOracle struct {
	head                 *types.Header
	hashByNum            map[uint64]common.Hash
	earliestIndexedBlock *types.Header
	blockByHashFn        BlockByHashFn
}

func NewCanonicalBlockHeaderOracle(head *types.Header, blockByHashFn BlockByHashFn) *CanonicalBlockHeaderOracle {
	return &CanonicalBlockHeaderOracle{
		head: head,
		hashByNum: map[uint64]common.Hash{
			head.Number.Uint64(): head.Hash(),
		},
		earliestIndexedBlock: head,
		blockByHashFn:        blockByHashFn,
	}
}

func (o *CanonicalBlockHeaderOracle) CurrentHeader() *types.Header {
	return o.head
}

// GetHeaderByNumber walks back from the current head to the requested block number
func (o *CanonicalBlockHeaderOracle) GetHeaderByNumber(n uint64) *types.Header {
	if o.head.Number.Uint64() < n {
		return nil
	}

	if o.earliestIndexedBlock.Number.Uint64() <= n {
		// guaranteed to be cached during lookup
		hash, ok := o.hashByNum[n]
		if !ok {
			panic(fmt.Errorf("block %v was not indexed when earliest block number is %v", n, o.earliestIndexedBlock.Number))
		}
		return o.blockByHashFn(hash).Header()
	}

	h := o.earliestIndexedBlock
	for h.Number.Uint64() > n {
		hash := h.ParentHash
		h = o.blockByHashFn(hash).Header()
		o.hashByNum[h.Number.Uint64()] = hash
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
		if h.Number.Uint64() == 0 {
			// Reachable if there aren't any cached blocks at or before the common ancestor
			break
		}
		h = o.blockByHashFn(h.ParentHash).Header()
	}
	o.earliestIndexedBlock = h
	return head.Hash()
}
