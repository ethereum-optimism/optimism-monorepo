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

	"github.com/ethereum-optimism/optimism/cannon/mipsevm/arch"
	"github.com/stretchr/testify/require"
)

// These tests are mostly copied from memory_test.go. With a few tweaks for 64-bit.

func TestMemory64MerkleProof(t *testing.T) {
	t.Run("nearly empty tree", func(t *testing.T) {
		m := NewMemory()
		m.SetWord(0x10000, 0xAABBCCDD_EEFF1122)
		proof := m.MerkleProof(0x10000)
		require.Equal(t, uint64(0xAABBCCDD_EEFF1122), binary.BigEndian.Uint64(proof[:8]))
		for i := 0; i < 64-5; i++ {
			require.Equal(t, zeroHashes[i][:], proof[32+i*32:32+i*32+32], "empty siblings")
		}
	})
	t.Run("fuller tree", func(t *testing.T) {
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
}

func TestMemory64MerkleRoot(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		m := NewMemory()
		root := m.MerkleRoot()
		require.Equal(t, zeroHashes[64-5], root, "fully zeroed memory should have expected zero hash")
	})
	t.Run("empty page", func(t *testing.T) {
		m := NewMemory()
		m.SetWord(0xF000, 0)
		root := m.MerkleRoot()
		require.Equal(t, zeroHashes[64-5], root, "fully zeroed memory should have expected zero hash")
	})
	t.Run("single page", func(t *testing.T) {
		m := NewMemory()
		m.SetWord(0xF000, 1)
		root := m.MerkleRoot()
		require.NotEqual(t, zeroHashes[64-5], root, "non-zero memory")
	})
	t.Run("repeat zero", func(t *testing.T) {
		m := NewMemory()
		m.SetWord(0xF000, 0)
		m.SetWord(0xF008, 0)
		root := m.MerkleRoot()
		require.Equal(t, zeroHashes[64-5], root, "zero still")
	})
	t.Run("two empty pages", func(t *testing.T) {
		m := NewMemory()
		m.SetWord(PageSize*3, 0)
		m.SetWord(PageSize*10, 0)
		root := m.MerkleRoot()
		require.Equal(t, zeroHashes[64-5], root, "zero still")
	})
	t.Run("random few pages", func(t *testing.T) {
		m := NewMemory()
		m.SetWord(PageSize*3, 1)
		m.SetWord(PageSize*5, 42)
		m.SetWord(PageSize*6, 123)
		p3 := m.MerkleizeSubtree((1 << PageKeySize) | 3)
		p5 := m.MerkleizeSubtree((1 << PageKeySize) | 5)
		p6 := m.MerkleizeSubtree((1 << PageKeySize) | 6)
		z := zeroHashes[PageAddrSize-5]
		r1 := HashPair(
			HashPair(
				HashPair(z, z),  // 0,1
				HashPair(z, p3), // 2,3
			),
			HashPair(
				HashPair(z, p5), // 4,5
				HashPair(p6, z), // 6,7
			),
		)
		r2 := m.MerkleizeSubtree(1 << (PageKeySize - 3))
		require.Equal(t, r1, r2, "expecting manual page combination to match subtree merkle func")
	})
	t.Run("invalidate page", func(t *testing.T) {
		m := NewMemory()
		m.SetWord(0xF000, 0)
		require.Equal(t, zeroHashes[64-5], m.MerkleRoot(), "zero at first")
		m.SetWord(0xF008, 1)
		require.NotEqual(t, zeroHashes[64-5], m.MerkleRoot(), "non-zero")
		m.SetWord(0xF008, 0)
		require.Equal(t, zeroHashes[64-5], m.MerkleRoot(), "zero again")
	})
}

func TestMemory64ReadWrite(t *testing.T) {
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
		m := NewMemory()
		data := []byte(strings.Repeat("under the big bright yellow sun ", 40))
		require.NoError(t, m.SetMemoryRange(0x1337, bytes.NewReader(data)))
		res, err := io.ReadAll(m.ReadMemoryRange(0x1337-10, Word(len(data)+20)))
		require.NoError(t, err)
		require.Equal(t, make([]byte, 10), res[:10], "empty start")
		require.Equal(t, data, res[10:len(res)-10], "result")
		require.Equal(t, make([]byte, 10), res[len(res)-10:], "empty end")
	})

	t.Run("empty range", func(t *testing.T) {
		m := NewMemory()
		addr := Word(0xAABBCC00)
		r := bytes.NewReader(nil)
		pre := m.MerkleRoot()
		preJSON, err := m.MarshalJSON()
		require.NoError(t, err)
		var preSerialized bytes.Buffer
		require.NoError(t, m.Serialize(&preSerialized))

		require.NoError(t, m.SetMemoryRange(addr, r))
		v := m.GetWord(0)
		require.Equal(t, Word(0), v)
		post := m.MerkleRoot()
		require.Equal(t, pre, post)

		// Assert that there are no extra zero pages in serialization
		postJSON, err := m.MarshalJSON()
		require.NoError(t, err)
		require.Equal(t, preJSON, postJSON)

		var postSerialized bytes.Buffer
		require.NoError(t, m.Serialize(&postSerialized))
		require.Equal(t, preSerialized.Bytes(), postSerialized.Bytes())
	})

	t.Run("range page overlap", func(t *testing.T) {
		m := NewMemory()
		data := bytes.Repeat([]byte{0xAA}, PageAddrSize)
		require.NoError(t, m.SetMemoryRange(0, bytes.NewReader(data)))
		for i := 0; i < PageAddrSize/arch.WordSizeBytes; i++ {
			addr := Word(i * arch.WordSizeBytes)
			require.Equal(t, Word(0xAAAAAAAA_AAAAAAAA), m.GetWord(addr))
		}

		data = []byte{0x11, 0x22, 0x33, 0x44}
		require.NoError(t, m.SetMemoryRange(0, bytes.NewReader(data)))
		require.Equal(t, Word(0x11223344_AAAAAAAA), m.GetWord(0))
		for i := 1; i < PageAddrSize/arch.WordSizeBytes; i++ {
			addr := Word(i * arch.WordSizeBytes)
			require.Equal(t, Word(0xAAAAAAAA_AAAAAAAA), m.GetWord(addr))
		}
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

func TestMemory64JSON(t *testing.T) {
	m := NewMemory()
	m.SetWord(8, 0xAABBCCDD_EEFF1122)
	dat, err := json.Marshal(m)
	require.NoError(t, err)
	var res Memory
	require.NoError(t, json.Unmarshal(dat, &res))
	require.Equal(t, Word(0xAABBCCDD_EEFF1122), res.GetWord(8))
}

func TestMemory64Copy(t *testing.T) {
	m := NewMemory()
	m.SetWord(0xAABBCCDD_8000, 0x000000_AABB)
	mcpy := m.Copy()
	require.Equal(t, Word(0xAABB), mcpy.GetWord(0xAABBCCDD_8000))
	require.Equal(t, m.MerkleRoot(), mcpy.MerkleRoot())
}
