package memory

import (
	"math/bits"
)

// BinaryTreeIndex is a representation of the state of the memory in a binary merkle tree.
type BinaryTreeIndex struct {
	// generalized index -> merkle root or nil if invalidated
	nodes map[uint64]*[32]byte
	// Reference to the page table from Memory.
	pageTable map[Word]*CachedPage
}

func NewBinaryTreeMemory() *Memory {
	pages := make(map[Word]*CachedPage)
	index := NewBinaryTreeIndex()
	index.setPageBacking(pages)
	return &Memory{
		merkleIndex:  index,
		pageTable:    pages,
		lastPageKeys: [2]Word{^Word(0), ^Word(0)}, // default to invalid keys, to not match any pages
	}
}

func NewBinaryTreeIndex() *BinaryTreeIndex {
	return &BinaryTreeIndex{
		nodes: make(map[uint64]*[32]byte),
	}
}
func (m *BinaryTreeIndex) setPageBacking(pages map[Word]*CachedPage) {
	m.pageTable = pages
}

func (m *BinaryTreeIndex) New() PageIndex {
	x := NewBinaryTreeIndex()
	return x
}

func (m *BinaryTreeIndex) Invalidate(addr Word) {
	// find the gindex of the first page covering the address: i.e. ((1 << WordSize) | addr) >> PageAddrSize
	// Avoid 64-bit overflow by distributing the right shift across the OR.
	gindex := (uint64(1) << (WordSize - PageAddrSize)) | uint64(addr>>PageAddrSize)

	for gindex > 0 {
		m.nodes[gindex] = nil
		gindex >>= 1
	}
}

func (m *BinaryTreeIndex) MerkleizeSubtree(gindex uint64) [32]byte {
	l := uint64(bits.Len64(gindex))
	if l > MemProofLeafCount {
		panic("gindex too deep")
	}
	if l > PageKeySize {
		depthIntoPage := l - 1 - PageKeySize
		pageIndex := (gindex >> depthIntoPage) & PageKeyMask
		if p, ok := m.pageTable[Word(pageIndex)]; ok {
			pageGindex := (1 << depthIntoPage) | (gindex & ((1 << depthIntoPage) - 1))
			return p.MerkleizeSubtree(pageGindex)
		} else {
			return zeroHashes[MemProofLeafCount-l] // page does not exist
		}
	}
	n, ok := m.nodes[gindex]
	if !ok {
		// if the node doesn't exist, the whole sub-tree is zeroed
		return zeroHashes[MemProofLeafCount-l]
	}
	if n != nil {
		return *n
	}
	left := m.MerkleizeSubtree(gindex << 1)
	right := m.MerkleizeSubtree((gindex << 1) | 1)
	r := HashPair(left, right)
	m.nodes[gindex] = &r
	return r
}

func (m *BinaryTreeIndex) MerkleProof(addr Word) (out [MemProofSize]byte) {
	proof := m.traverseBranch(1, addr, 0)
	// encode the proof
	for i := 0; i < MemProofLeafCount; i++ {
		copy(out[i*32:(i+1)*32], proof[i][:])
	}
	return out
}

func (m *BinaryTreeIndex) traverseBranch(parent uint64, addr Word, depth uint8) (proof [][32]byte) {
	if depth == WordSize-5 {
		proof = make([][32]byte, 0, WordSize-5+1)
		proof = append(proof, m.MerkleizeSubtree(parent))
		return
	}
	if depth > WordSize-5 {
		panic("traversed too deep")
	}
	self := parent << 1
	sibling := self | 1
	if addr&(1<<((WordSize-1)-depth)) != 0 {
		self, sibling = sibling, self
	}
	proof = m.traverseBranch(self, addr, depth+1)
	siblingNode := m.MerkleizeSubtree(sibling)
	proof = append(proof, siblingNode)
	return
}

func (m *BinaryTreeIndex) MerkleRoot() [32]byte {
	return m.MerkleizeSubtree(1)
}

func (m *BinaryTreeIndex) AddPage(pageIndex Word) {
	// make nodes to root
	k := (1 << PageKeySize) | uint64(pageIndex)
	for k > 0 {
		m.nodes[k] = nil
		k >>= 1
	}
}
