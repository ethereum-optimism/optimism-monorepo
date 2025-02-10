//go:build cannon64
// +build cannon64

package memory

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMemory64TrieMerkleProof(t *testing.T) {
	t.Run("nearly empty tree", func(t *testing.T) {
		m := NewTrieMemory()
		m.SetWord(0x10000, 0xAABBCCDD_EEFF1122)
		proof := m.MerkleProof(0x10000)
		require.Equal(t, uint64(0xAABBCCDD_EEFF1122), binary.BigEndian.Uint64(proof[:8]))
		for i := 0; i < 64-5; i++ {
			require.Equal(t, zeroHashes[i][:], proof[32+i*32:32+i*32+32], "empty siblings")
		}
	})
	t.Run("NewTrieMemory tree", func(t *testing.T) {
		m := NewMemory()
		m.SetWord(0x10000, 0xaabbccdd)
		m.SetWord(0x80008, 42)
		m.SetWord(0x13370000, 123)
		root := m.MerkleRoot()
		proof := m.MerkleProof(0x80008)
		require.Equal(t, uint64(42), binary.BigEndian.Uint64(proof[8:16]))
		node := *(*[32]byte)(proof[:32])
		path := uint32(0x80008) >> 5
		for i := 32; i < len(proof); i += 32 {
			sib := *(*[32]byte)(proof[i : i+32])
			if path&1 != 0 {
				node = HashPair(sib, node)
			} else {
				node = HashPair(node, sib)
			}
			path >>= 1
		}
		require.Equal(t, root, node, "proof must verify")
	})

	t.Run("consistency test", func(t *testing.T) {
		m := NewTrieMemory()
		addr := uint64(0x1234560000000)
		m.SetWord(addr, 1)
		proof1 := m.MerkleProof(addr)
		proof2 := m.MerkleProof(addr)
		require.Equal(t, proof1, proof2, "Proofs for the same address should be consistent")
	})

	t.Run("stress test", func(t *testing.T) {
		m := NewTrieMemory()
		var addresses []uint64
		for i := uint64(0); i < 10000; i++ {
			addr := i * 0x1000000 // Spread out addresses
			addresses = append(addresses, addr)
			m.SetWord(addr, Word(i+1))
		}
		root := m.MerkleRoot()
		for _, addr := range addresses {
			proof := m.MerkleProof(addr)
			verifyProof(t, root, proof, addr)
		}
	})

	t.Run("multiple levels", func(t *testing.T) {
		m := NewTrieMemory()
		addresses := []uint64{
			0x0000000000000,
			0x0400000000000,
			0x0800000000000,
			0x0C00000000000,
			0x1000000000000,
			0x1400000000000,
		}
		for i, addr := range addresses {
			m.SetWord(addr, Word(i+1))
		}
		root := m.MerkleRoot()
		for _, addr := range addresses {
			proof := m.MerkleProof(addr)
			verifyProof(t, root, proof, addr)
		}
	})

	t.Run("sparse tree", func(t *testing.T) {
		m := NewTrieMemory()
		addresses := []uint64{
			0x0000000000000,
			0x0000400000000,
			0x0004000000000,
			0x0040000000000,
			0x0400000000000,
			0x3C00000000000,
		}
		for i, addr := range addresses {
			m.SetWord(addr, Word(i+1))
		}
		root := m.MerkleRoot()
		for _, addr := range addresses {
			proof := m.MerkleProof(addr)
			verifyProof(t, root, proof, addr)
		}
	})

	t.Run("large addresses", func(t *testing.T) {
		m := NewTrieMemory()
		addresses := []uint64{
			0x10_00_00_00_00_00_00_00,
			0x10_00_00_00_00_00_00_08,
			0x10_00_00_00_00_00_00_10,
			0x10_00_00_00_00_00_00_18,
		}
		for i, addr := range addresses {
			m.SetWord(addr, Word(i+1))
		}
		root := m.MerkleRoot()
		for _, addr := range addresses {
			proof := m.MerkleProof(addr)
			verifyProof(t, root, proof, addr)
		}
	})
}
func TestMerkleProofWithPartialPaths(t *testing.T) {
	testCases := []struct {
		name        string
		setupMemory func(*Memory)
		proofAddr   uint64
	}{
		{
			name: "Path ends at level 1",
			setupMemory: func(m *Memory) {
				m.SetWord(0x10_00_00_00_00_00_00_00, 1)
			},
			proofAddr: 0x20_00_00_00_00_00_00_00,
		},
		{
			name: "Path ends at level 2",
			setupMemory: func(m *Memory) {
				m.SetWord(0x10_00_00_00_00_00_00_00, 1)
			},
			proofAddr: 0x11_00_00_00_00_00_00_00,
		},
		{
			name: "Path ends at level 3",
			setupMemory: func(m *Memory) {
				m.SetWord(0x10_10_00_00_00_00_00_00, 1)
			},
			proofAddr: 0x10_11_00_00_00_00_00_00,
		},
		{
			name: "Path ends at level 4",
			setupMemory: func(m *Memory) {
				m.SetWord(0x10_10_10_00_00_00_00_00, 1)
			},
			proofAddr: 0x10_10_11_00_00_00_00_00,
		},
		{
			name: "Full path to level 5, page doesn't exist",
			setupMemory: func(m *Memory) {
				m.SetWord(0x10_10_10_10_00_00_00_00, 1)
			},
			proofAddr: 0x10_10_10_10_10_00_00_00, // Different page in the same level 5 node
		},
		{
			name: "Path ends at level 3, check different page offsets",
			setupMemory: func(m *Memory) {
				m.SetWord(0x10_10_00_00_00_00_00_00, 1)
				m.SetWord(0x10_10_00_00_00_00_10_00, 2)
			},
			proofAddr: 0x10_10_00_00_00_00_20_00, // Different offset in the same page
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m := NewTrieMemory()
			tc.setupMemory(m)

			proof := m.MerkleProof(tc.proofAddr)

			// Check that the proof is filled correctly
			verifyProof(t, m.MerkleRoot(), proof, tc.proofAddr)
			//checkProof(t, proof, tc.expectedDepth)
		})
	}
}

func verifyProof(t *testing.T, expectedRoot [32]byte, proof [MemProofSize]byte, addr uint64) {
	node := *(*[32]byte)(proof[:32])
	path := addr >> 5
	for i := 32; i < len(proof); i += 32 {
		sib := *(*[32]byte)(proof[i : i+32])
		if path&1 != 0 {
			node = HashPair(sib, node)
		} else {
			node = HashPair(node, sib)
		}
		path >>= 1
	}
	require.Equal(t, expectedRoot, node, "proof must verify for address 0x%x", addr)
}

func TestMemory64TrieMerkleRoot(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		m := NewTrieMemory()
		root := m.MerkleRoot()
		require.Equal(t, zeroHashes[64-5], root, "fully zeroed memory should have expected zero hash")
	})
	t.Run("empty page", func(t *testing.T) {
		m := NewTrieMemory()
		m.SetWord(0xF000, 0)
		root := m.MerkleRoot()
		require.Equal(t, zeroHashes[64-5], root, "fully zeroed memory should have expected zero hash")
	})
	t.Run("single page", func(t *testing.T) {
		m := NewTrieMemory()
		m.SetWord(0xF000, 1)
		root := m.MerkleRoot()
		require.NotEqual(t, zeroHashes[64-5], root, "non-zero memory")
	})
	t.Run("repeat zero", func(t *testing.T) {
		m := NewTrieMemory()
		m.SetWord(0xF000, 0)
		m.SetWord(0xF008, 0)
		root := m.MerkleRoot()
		require.Equal(t, zeroHashes[64-5], root, "zero still")
	})
	t.Run("two empty pages", func(t *testing.T) {
		m := NewTrieMemory()
		m.SetWord(PageSize*3, 0)
		m.SetWord(PageSize*10, 0)
		root := m.MerkleRoot()
		require.Equal(t, zeroHashes[64-5], root, "zero still")
	})

	t.Run("random few pages", func(t *testing.T) {
		m := NewTrieMemory()
		index, ok := m.merkleIndex.(*TrieIndex)
		require.True(t, ok, "should be TrieIndex")
		m.SetWord(PageSize*3, 1)
		m.SetWord(PageSize*5, 42)
		m.SetWord(PageSize*6, 123)

		p0 := index.radix.MerkleizeNode(0, 8)
		p1 := index.radix.MerkleizeNode(0, 9)
		p2 := index.radix.MerkleizeNode(0, 10)
		p3 := index.radix.MerkleizeNode(0, 11)
		p4 := index.radix.MerkleizeNode(0, 12)
		p5 := index.radix.MerkleizeNode(0, 13)
		p6 := index.radix.MerkleizeNode(0, 14)
		p7 := index.radix.MerkleizeNode(0, 15)

		r1 := HashPair(
			HashPair(
				HashPair(p0, p1), // 0,1
				HashPair(p2, p3), // 2,3
			),
			HashPair(
				HashPair(p4, p5), // 4,5
				HashPair(p6, p7), // 6,7
			),
		)
		r2 := m.MerkleRoot()
		require.Equal(t, r1, r2, "expecting manual page combination to match subtree merkle func")
	})

	t.Run("invalidate page", func(t *testing.T) {
		m := NewTrieMemory()
		m.SetWord(0xF000, 0)
		require.Equal(t, zeroHashes[64-5], m.MerkleRoot(), "zero at first")
		m.SetWord(0xF008, 2)
		require.NotEqual(t, zeroHashes[64-5], m.MerkleRoot(), "non-zero")
		m.SetWord(0xF008, 0)
		require.Equal(t, zeroHashes[64-5], m.MerkleRoot(), "zero again")
	})
}

func TestMemory64TrieReadWrite(t *testing.T) {
	t.Run("large random", func(t *testing.T) {
		m := NewMemory()
		data := make([]byte, 20_000)
		_, err := rand.Read(data[:])
		require.NoError(t, err)
		require.NoError(t, m.SetMemoryRange(0, bytes.NewReader(data)))
		for _, i := range []Word{0, 8, 1000, 20_000 - 8} {
			v := m.GetWord(i)
			expected := binary.BigEndian.Uint64(data[i : i+8])
			require.Equalf(t, expected, v, "read at %d", i)
		}
	})

	t.Run("repeat range", func(t *testing.T) {
		m := NewTrieMemory()
		data := []byte(strings.Repeat("under the big bright yellow sun ", 40))
		require.NoError(t, m.SetMemoryRange(0x1337, bytes.NewReader(data)))
		res, err := io.ReadAll(m.ReadMemoryRange(0x1337-10, uint64(len(data)+20)))
		require.NoError(t, err)
		require.Equal(t, make([]byte, 10), res[:10], "empty start")
		require.Equal(t, data, res[10:len(res)-10], "result")
		require.Equal(t, make([]byte, 10), res[len(res)-10:], "empty end")
	})

	t.Run("read-write", func(t *testing.T) {
		m := NewMemory()
		m.SetWord(16, 0xAABBCCDD_EEFF1122)
		require.Equal(t, Word(0xAABBCCDD_EEFF1122), m.GetWord(16))
		m.SetWord(16, 0xAABB1CDD_EEFF1122)
		require.Equal(t, Word(0xAABB1CDD_EEFF1122), m.GetWord(16))
		m.SetWord(16, 0xAABB1CDD_EEFF1123)
		require.Equal(t, Word(0xAABB1CDD_EEFF1123), m.GetWord(16))
	})

	t.Run("unaligned read", func(t *testing.T) {
		m := NewMemory()
		m.SetWord(16, Word(0xAABBCCDD_EEFF1122))
		m.SetWord(24, 0x11223344_55667788)
		for i := Word(17); i < 24; i++ {
			require.Panics(t, func() {
				m.GetWord(i)
			})
		}
		require.Equal(t, Word(0x11223344_55667788), m.GetWord(24))
		require.Equal(t, Word(0), m.GetWord(32))
		require.Equal(t, Word(0xAABBCCDD_EEFF1122), m.GetWord(16))
	})

	t.Run("unaligned write", func(t *testing.T) {
		m := NewMemory()
		m.SetWord(16, 0xAABBCCDD_EEFF1122)
		require.Panics(t, func() {
			m.SetWord(17, 0x11223344)
		})
		require.Panics(t, func() {
			m.SetWord(18, 0x11223344)
		})
		require.Panics(t, func() {
			m.SetWord(19, 0x11223344)
		})
		require.Panics(t, func() {
			m.SetWord(20, 0x11223344)
		})
		require.Panics(t, func() {
			m.SetWord(21, 0x11223344)
		})
		require.Panics(t, func() {
			m.SetWord(22, 0x11223344)
		})
		require.Panics(t, func() {
			m.SetWord(23, 0x11223344)
		})
		require.Equal(t, Word(0xAABBCCDD_EEFF1122), m.GetWord(16))
	})
}

func TestMemory64TrieJSON(t *testing.T) {
	m := NewTrieMemory()
	m.SetWord(8, 0xAABBCCDD_EEFF1122)
	dat, err := json.Marshal(m)
	require.NoError(t, err)
	res := NewMemory()
	require.NoError(t, json.Unmarshal(dat, &res))
	require.Equal(t, Word(0xAABBCCDD_EEFF1122), res.GetWord(8))
}
