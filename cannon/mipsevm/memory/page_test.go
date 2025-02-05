package memory

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestCachedPage(t *testing.T) {
	p := &CachedPage{Data: new(Page)}
	p.Data[42] = 0xab

	gindex := ((uint64(1) << PageAddrSize) | 42) >> 5
	node := common.Hash(p.MerkleizeSubtree(gindex))
	expectedLeaf := common.Hash{10: 0xab}
	require.Equal(t, expectedLeaf, node, "leaf nodes should not be hashed")

	node = p.MerkleizeSubtree(gindex >> 1)
	expectedParent := common.Hash(HashPair(zeroHashes[0], expectedLeaf))
	require.Equal(t, expectedParent, node, "can get the parent node")

	node = p.MerkleizeSubtree(gindex >> 2)
	expectedParentParent := common.Hash(HashPair(expectedParent, zeroHashes[1]))
	require.Equal(t, expectedParentParent, node, "and the parent of the parent")

	pre := p.MerkleRoot()
	p.Data[42] = 0xcd
	post := p.MerkleRoot()
	require.Equal(t, pre, post, "no change expected until cache is invalidated")

	p.invalidate(42)
	post2 := p.MerkleRoot()
	require.NotEqual(t, post, post2, "change after cache invalidation")

	p.Data[2000] = 0xef
	p.invalidate(42)
	post3 := p.MerkleRoot()
	require.Equal(t, post2, post3, "local invalidation is not global invalidation")

	p.invalidate(2000)
	post4 := p.MerkleRoot()
	require.NotEqual(t, post3, post4, "can see the change now")

	p.Data[1000] = 0xff
	p.InvalidateFull()
	post5 := p.MerkleRoot()
	require.NotEqual(t, post4, post5, "and global invalidation works regardless of changed data")
}
