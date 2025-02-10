//go:build cannon64
// +build cannon64

package memory

import (
	"math/bits"
)

// Representation of the state of the memory as a Merkle Patricia Trie (MPT). Only for 64 bit Cannon.
//
// This implementation is copied from asterisc.
type TrieIndex struct {
	radix         *L1
	branchFactors [6]uint64

	// Note: since we don't de-alloc pageTable, we don't do ref-counting.
	// Once a page exists, it doesn't leave memory
	pageTable map[Word]*CachedPage
}

func NewTrieIndex() *TrieIndex {
	node := &L1{}
	return &TrieIndex{
		radix:         node,
		pageTable:     make(map[Word]*CachedPage),
		branchFactors: [6]uint64{16, 16, 6, 6, 4, 4},
	}
}

func (m *TrieIndex) setPageBacking(pages map[Word]*CachedPage) {
	m.pageTable = pages
}

func (m *TrieIndex) New() PageIndex {
	x := NewTrieIndex()
	return x
}

// RadixNode is an interface defining the operations for a node in a radix trie.
type RadixNode interface {
	// GenerateProof generates the Merkle proof for the given address.
	GenerateProof(addr Word, proofs [][32]byte)
	// MerkleizeNode computes the Merkle root hash for the node at the given generalized index.
	MerkleizeNode(addr Word, gindex uint64) [32]byte
}

// SmallRadixNode is a radix trie node with a branching factor of 4 bits.
type SmallRadixNode[C RadixNode] struct {
	Children   [1 << 4]*C       // Array of child nodes, indexed by 4-bit keys.
	Hashes     [1 << 4][32]byte // Cached hashes for intermediate hash node.
	HashExists uint16           // Bitmask indicating if the intermediate hash exist (1 bit per intermediate node).
	HashValid  uint16           // Bitmask indicating if the intermediate hashes are valid (1 bit per intermediate node).
	Depth      uint64           // The depth of this node in the trie (number of bits from the root).
}

// MediumRadixNode is a radix trie node with a branching factor of 6 bits.
type MediumRadixNode[C RadixNode] struct {
	Children   [1 << 6]*C // Array of child nodes, indexed by 6-bit keys.
	Hashes     [1 << 6][32]byte
	HashExists uint64
	HashValid  uint64
	Depth      uint64
}

// LargeRadixNode is a radix trie node with a branching factor of 16 bits.
type LargeRadixNode[C RadixNode] struct {
	Children   [1 << 16]*C // Array of child nodes, indexed by 16-bit keys.
	Hashes     [1 << 16][32]byte
	HashExists [(1 << 16) / 64]uint64
	HashValid  [(1 << 16) / 64]uint64
	Depth      uint64
}

// Define a sequence of radix trie node types (L1 to L7) representing different levels in the trie.
// Each level corresponds to a node type, where L1 is the root node and L7 is the leaf level pointing to Memory.
// The cumulative bit-lengths of the addresses represented by the nodes from L1 to L7 add up to 52 bits.

type L1 = LargeRadixNode[L2]
type L2 = *LargeRadixNode[L3]
type L3 = *MediumRadixNode[L4]
type L4 = *MediumRadixNode[L5]
type L5 = *SmallRadixNode[L6]
type L6 = *SmallRadixNode[L7]
type L7 = *TrieIndex

// InvalidateNode invalidates the hash cache along the path to the specified address.
// It marks the necessary intermediate hashes as invalid, forcing them to be recomputed when needed.
func (n *SmallRadixNode[C]) InvalidateNode(gindex uint64) {
	branchIdx := (gindex + 1<<4) / 2 // Compute the index for the hash tree traversal.

	// Traverse up the hash tree, invalidating hashes along the way.
	for index := branchIdx; index > 0; index >>= 1 {
		hashBit := index & 15          // Get the relevant bit position (0-15).
		n.HashExists |= 1 << hashBit   // Mark the intermediate hash path as existing.
		n.HashValid &= ^(1 << hashBit) // Invalidate the hash at this position.
	}
}

func (n *MediumRadixNode[C]) InvalidateNode(gindex uint64) {
	branchIdx := (gindex + 1<<6) / 2

	for index := branchIdx; index > 0; index >>= 1 {
		hashBit := index & 63
		n.HashExists |= 1 << hashBit
		n.HashValid &= ^(1 << hashBit)
	}
}

func (n *LargeRadixNode[C]) InvalidateNode(gindex uint64) {
	branchIdx := (gindex + 1<<16) / 2

	for index := branchIdx; index > 0; index >>= 1 {
		hashIndex := index >> 6
		hashBit := index & 63
		n.HashExists[hashIndex] |= 1 << hashBit
		n.HashValid[hashIndex] &= ^(1 << hashBit)
	}
}

// GenerateProof generates the Merkle proof for the given address.
// It collects the necessary sibling hashes along the path to reconstruct the Merkle proof.
func (n *SmallRadixNode[C]) GenerateProof(addr Word, proofs [][32]byte) {
	path := addressToRadixPath(addr, n.Depth, 4)

	if n.Children[path] == nil {
		// When no child exists at this path, the rest of the proofs are zero hashes.
		fillZeroHashRange(proofs, 0, 60-n.Depth-4)
	} else {
		// Recursively generate proofs from the child node.
		(*n.Children[path]).GenerateProof(addr, proofs)
	}

	// Collect sibling hashes along the path for the proof.
	proofIndex := 60 - n.Depth - 4
	for idx := path + 1<<4; idx > 1; idx >>= 1 {
		sibling := idx ^ 1 // Get the sibling index.
		proofs[proofIndex] = n.MerkleizeNode(addr>>(64-n.Depth), sibling)
		proofIndex += 1
	}
}

func (n *MediumRadixNode[C]) GenerateProof(addr Word, proofs [][32]byte) {
	path := addressToRadixPath(addr, n.Depth, 6)

	if n.Children[path] == nil {
		fillZeroHashRange(proofs, 0, 60-n.Depth-6)
	} else {
		(*n.Children[path]).GenerateProof(addr, proofs)
	}

	proofIndex := 60 - n.Depth - 6
	for idx := path + 1<<6; idx > 1; idx >>= 1 {
		sibling := idx ^ 1
		proofs[proofIndex] = n.MerkleizeNode(addr>>(64-n.Depth), sibling)
		proofIndex += 1
	}
}

func (n *LargeRadixNode[C]) GenerateProof(addr Word, proofs [][32]byte) {
	path := addressToRadixPath(addr, n.Depth, 16)

	if n.Children[path] == nil {
		fillZeroHashRange(proofs, 0, 60-n.Depth-16)
	} else {
		(*n.Children[path]).GenerateProof(addr, proofs)
	}
	proofIndex := 60 - n.Depth - 16
	for idx := path + 1<<16; idx > 1; idx >>= 1 {
		sibling := idx ^ 1
		proofs[proofIndex] = n.MerkleizeNode(addr>>(64-n.Depth), sibling)
		proofIndex += 1
	}
}

func (m *TrieIndex) GenerateProof(addr Word, proofs [][32]byte) {
	pageIndex := addr >> PageAddrSize

	// number of proof for a page is 8
	// 0: leaf page data, 7: page's root
	if p, ok := m.pageTable[pageIndex]; ok {
		pageProofs := p.GenerateProof(addr) // Generate proof from the page.
		copy(proofs[:8], pageProofs)
	} else {
		fillZeroHashRange(proofs, 0, 8) // Return zero hashes if the page does not exist.
	}
}

// MerkleizeNode computes the Merkle root hash for the node at the given generalized index.
// It recursively computes the hash of the subtree rooted at the given index.
// Note: The 'addr' parameter represents the partial address accumulated up to this node, not the full address. It represents the path taken in the trie to reach this node.
func (n *SmallRadixNode[C]) MerkleizeNode(addr Word, gindex uint64) [32]byte {
	depth := uint64(bits.Len64(gindex)) // Get the depth of the current gindex.

	if depth > 5 {
		panic("gindex too deep")
	}

	// Leaf node of the radix trie (17~32)
	if depth > 4 {
		childIndex := gindex - 1<<4
		if n.Children[childIndex] == nil {
			// Return zero hash if child does not exist.
			return zeroHashes[64-5+1-(depth+n.Depth)]
		}

		// Update the partial address by appending the child index bits.
		// This accumulates the address as we traverse deeper into the trie.
		addr <<= 4
		addr |= Word(childIndex)
		return (*n.Children[childIndex]).MerkleizeNode(addr, 1)
	}

	// Intermediate node of the radix trie (0~15)
	hashBit := gindex & 15

	if (n.HashExists & (1 << hashBit)) != 0 {
		if (n.HashValid & (1 << hashBit)) != 0 {
			// Return the cached hash if valid.
			return n.Hashes[gindex]
		} else {
			left := n.MerkleizeNode(addr, gindex<<1)
			right := n.MerkleizeNode(addr, (gindex<<1)|1)

			// Hash the pair and cache the result.
			r := HashPair(left, right)
			n.Hashes[gindex] = r
			n.HashValid |= 1 << hashBit
			return r
		}
	} else {
		// Return zero hash for non-existent child.
		return zeroHashes[64-5+1-(depth+n.Depth)]
	}
}

func (n *MediumRadixNode[C]) MerkleizeNode(addr Word, gindex uint64) [32]byte {
	depth := uint64(bits.Len64(gindex)) // Get the depth of the current gindex.

	if depth > 7 {
		panic("gindex too deep")
	}

	// Leaf node of the radix trie (64~128)
	if depth > 6 {
		childIndex := gindex - 1<<6

		if n.Children[childIndex] == nil {
			// Return zero hash if child does not exist.
			return zeroHashes[64-5+1-(depth+n.Depth)]
		}

		// Update the partial address by appending the child index bits.
		// This accumulates the address as we traverse deeper into the trie.
		addr <<= 6
		addr |= Word(childIndex)
		return (*n.Children[childIndex]).MerkleizeNode(addr, 1)
	}

	// Intermediate node of the radix trie (0~16)
	hashBit := gindex & 63

	if (n.HashExists & (1 << hashBit)) != 0 {
		if (n.HashValid & (1 << hashBit)) != 0 {
			// Return the cached hash if valid.
			return n.Hashes[gindex]
		} else {
			left := n.MerkleizeNode(addr, gindex<<1)
			right := n.MerkleizeNode(addr, (gindex<<1)|1)

			// Hash the pair and cache the result.
			r := HashPair(left, right)
			n.Hashes[gindex] = r
			n.HashValid |= 1 << hashBit
			return r
		}
	} else {
		// Return zero hash for non-existent child.
		return zeroHashes[64-5+1-(depth+n.Depth)]
	}
}

func (n *LargeRadixNode[C]) MerkleizeNode(addr Word, gindex uint64) [32]byte {
	depth := uint64(bits.Len64(gindex))

	if depth > 17 {
		panic("gindex too deep")
	}

	// Leaf node of the radix trie (2^15~2^16)
	if depth > 16 {
		childIndex := gindex - 1<<16
		if n.Children[int(childIndex)] == nil {
			return zeroHashes[64-5+1-(depth+n.Depth)]
		}

		addr <<= 16
		addr |= Word(childIndex)
		return (*n.Children[childIndex]).MerkleizeNode(addr, 1)
	}

	// Intermediate node of the radix trie (0~2^15)
	hashIndex := gindex >> 6
	hashBit := gindex & 63
	if (n.HashExists[hashIndex] & (1 << hashBit)) != 0 {
		if (n.HashValid[hashIndex] & (1 << hashBit)) != 0 {
			return n.Hashes[gindex]
		} else {
			left := n.MerkleizeNode(addr, gindex<<1)
			right := n.MerkleizeNode(addr, (gindex<<1)|1)

			r := HashPair(left, right)
			n.Hashes[gindex] = r
			n.HashValid[hashIndex] |= 1 << hashBit
			return r
		}
	} else {
		return zeroHashes[64-5+1-(depth+n.Depth)]
	}
}

func (m *TrieIndex) MerkleizeNode(addr Word, gindex uint64) [32]byte {
	depth := uint64(bits.Len64(gindex))

	pageIndex := addr
	if p, ok := m.pageTable[pageIndex]; ok {
		return p.MerkleRoot()
	} else {
		return zeroHashes[64-5+1-(depth-1+52)]
	}
}

// MerkleRoot computes the Merkle root hash of the entire memory.
func (m *TrieIndex) MerkleRoot() [32]byte {
	return (*m.radix).MerkleizeNode(0, 1)
}

func (m *TrieIndex) MerkleizeSubtree(gindex uint64) [32]byte {
	return (*m.radix).MerkleizeNode(0, gindex)
}

// MerkleProof generates the Merkle proof for the specified address in memory.
func (m *TrieIndex) MerkleProof(addr Word) [MemProofSize]byte {
	proofs := make([][32]byte, 60)
	m.radix.GenerateProof(addr, proofs)
	return encodeProofs(proofs)
}

// zeroHashRange returns a slice of zero hashes from start to end.
func fillZeroHashRange(slice [][32]byte, start, end uint64) {
	if start == 0 {
		slice[0] = zeroHashes[0]
		start++
	}
	for i := start; i < end; i++ {
		slice[i] = zeroHashes[i-1]
	}
}

// encodeProofs encodes the list of proof hashes into a byte array.
func encodeProofs(proofs [][32]byte) [MemProofSize]byte {
	var out [MemProofSize]byte
	for i := 0; i < MemProofLeafCount; i++ {
		copy(out[i*32:(i+1)*32], proofs[i][:])
	}
	return out
}

// addressToRadixPath extracts a segment of bits from an address, starting from 'position' with 'count' bits.
// It returns the extracted bits as a uint64.
func addressToRadixPath(addr Word, position, count uint64) uint64 {
	// Calculate the total shift amount.
	totalShift := 64 - position - count

	// Shift the address to bring the desired bits to the LSB.
	addr >>= totalShift

	// Extract the desired bits using a mask.
	return uint64(addr) & ((1 << count) - 1)
}

// addressToRadixPaths converts an address into a slice of radix path indices based on the branch factors.
func (m *TrieIndex) addressToRadixPaths(addr Word) []uint64 {
	path := make([]uint64, len(m.branchFactors))
	var position uint64

	for index, branchFactor := range m.branchFactors {
		path[index] = addressToRadixPath(addr, position, branchFactor)
		position += branchFactor
	}

	return path
}

// AllocPage allocates a new page at the specified page index in memory.
func (m *TrieIndex) AddPage(pageIndex Word) {
	addr := pageIndex << PageAddrSize
	branchPaths := m.addressToRadixPaths(addr)
	depth := uint64(0)

	// Build the radix trie path to the new page, creating nodes as necessary.
	// This code is a bit repetitive, but better for the compiler to optimize.
	radixLevel1 := m.radix
	depth += m.branchFactors[0]
	if (*radixLevel1).Children[branchPaths[0]] == nil {
		node := &LargeRadixNode[L3]{Depth: depth}
		(*radixLevel1).Children[branchPaths[0]] = &node
	}
	(*radixLevel1).InvalidateNode(branchPaths[0])

	radixLevel2 := *(*radixLevel1).Children[branchPaths[0]]
	depth += m.branchFactors[1]
	if (radixLevel2).Children[branchPaths[1]] == nil {
		node := &MediumRadixNode[L4]{Depth: depth}
		(radixLevel2).Children[branchPaths[1]] = &node
	}
	(radixLevel2).InvalidateNode(branchPaths[1])

	radixLevel3 := *(*radixLevel2).Children[branchPaths[1]]
	depth += m.branchFactors[2]
	if (radixLevel3).Children[branchPaths[2]] == nil {
		node := &MediumRadixNode[L5]{Depth: depth}
		(radixLevel3).Children[branchPaths[2]] = &node
	}
	(radixLevel3).InvalidateNode(branchPaths[2])

	radixLevel4 := *(*radixLevel3).Children[branchPaths[2]]
	depth += m.branchFactors[3]
	if (radixLevel4).Children[branchPaths[3]] == nil {
		node := &SmallRadixNode[L6]{Depth: depth}
		(radixLevel4).Children[branchPaths[3]] = &node
	}
	(radixLevel4).InvalidateNode(branchPaths[3])

	radixLevel5 := *(*radixLevel4).Children[branchPaths[3]]
	depth += m.branchFactors[4]
	if (radixLevel5).Children[branchPaths[4]] == nil {
		node := &SmallRadixNode[L7]{Depth: depth}
		(radixLevel5).Children[branchPaths[4]] = &node
	}
	(radixLevel5).InvalidateNode(branchPaths[4])

	radixLevel6 := *(*radixLevel5).Children[branchPaths[4]]
	(radixLevel6).Children[branchPaths[5]] = &m
	(radixLevel6).InvalidateNode(branchPaths[5])
}

// Invalidate invalidates the cache along the path from the specified address up to the root.
// It ensures that any cached hashes are recomputed when needed.
func (m *TrieIndex) Invalidate(addr Word) {
	branchPaths := m.addressToRadixPaths(addr)

	currentLevel1 := m.radix
	currentLevel1.InvalidateNode(branchPaths[0])

	radixLevel2 := (*m.radix).Children[branchPaths[0]]
	if radixLevel2 == nil {
		return
	}
	(*radixLevel2).InvalidateNode(branchPaths[1])

	radixLevel3 := (*radixLevel2).Children[branchPaths[1]]
	if radixLevel3 == nil {
		return
	}
	(*radixLevel3).InvalidateNode(branchPaths[2])

	radixLevel4 := (*radixLevel3).Children[branchPaths[2]]
	if radixLevel4 == nil {
		return
	}
	(*radixLevel4).InvalidateNode(branchPaths[3])

	radixLevel5 := (*radixLevel4).Children[branchPaths[3]]
	if radixLevel5 == nil {
		return
	}
	(*radixLevel5).InvalidateNode(branchPaths[4])

	radixLevel6 := (*radixLevel5).Children[branchPaths[4]]
	if radixLevel6 == nil {
		return
	}
	(*radixLevel6).InvalidateNode(branchPaths[5])
}
