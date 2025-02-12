//go:build cannon64
// +build cannon64

package memory

import (
	"testing"

	"math/rand"

	"github.com/ethereum-optimism/optimism/cannon/mipsevm/arch"
)

const (
	smallDataset  = 12_500_000
	mediumDataset = 100_000_000
	largeDataset  = 400_000_000
)

func BenchmarkMemoryOperations(b *testing.B) {
	benchmarks := []struct {
		name string
		fn   func(b *testing.B, m *Memory)
	}{
		{"RandomReadWrite_Small", benchRandomReadWrite(smallDataset)},
		{"RandomReadWrite_Medium", benchRandomReadWrite(mediumDataset)},
		{"RandomReadWrite_Large", benchRandomReadWrite(largeDataset)},
		{"SequentialReadWrite_Small", benchSequentialReadWrite(smallDataset)},
		{"SequentialReadWrite_Large", benchSequentialReadWrite(largeDataset)},
		{"SparseMemoryUsage", benchSparseMemoryUsage},
		{"DenseMemoryUsage", benchDenseMemoryUsage},
		{"SmallFrequentUpdates", benchSmallFrequentUpdates},
		{"MerkleProofGeneration_Small", benchMerkleProofGeneration(smallDataset)},
		{"MerkleProofGeneration_Large", benchMerkleProofGeneration(largeDataset)},
		{"MerkleRootCalculation_Small", benchMerkleRootCalculation(smallDataset)},
		{"MerkleRootCalculation_Large", benchMerkleRootCalculation(largeDataset)},
	}

	for _, bm := range benchmarks {
		b.Run("BinaryTree", func(b *testing.B) {
			b.Run(bm.name, func(b *testing.B) {
				m := NewBinaryTreeMemory()
				b.ResetTimer()
				bm.fn(b, m)
			})
		})
		b.Run("Trie", func(b *testing.B) {
			b.Run((bm.name), func(b *testing.B) {
				m := NewTrieMemory()
				b.ResetTimer()
				bm.fn(b, m)
			})
		})
	}
}

func benchRandomReadWrite(size int) func(b *testing.B, m *Memory) {
	return func(b *testing.B, m *Memory) {
		addresses := make([]uint64, size)
		for i := range addresses {
			addresses[i] = rand.Uint64() & arch.AddressMask
		}
		data := Word(0x1234567890ABCDEF)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			addr := addresses[i%len(addresses)]
			if i%2 == 0 {
				m.SetWord(addr, data)
			} else {
				data = m.GetWord(addr)
			}
		}
	}
}

func benchSequentialReadWrite(size int) func(b *testing.B, m *Memory) {
	return func(b *testing.B, m *Memory) {
		data := Word(0x1234567890ABCDEF)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			addr := Word((i % size) * 8)
			if i%2 == 0 {
				m.SetWord(addr, data)
			} else {
				data = m.GetWord(addr)
			}
		}
	}
}

func benchSparseMemoryUsage(b *testing.B, m *Memory) {
	data := Word(0x1234567890ABCDEF)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		addr := (uint64(i) * 10_000_000) & arch.AddressMask // Large gaps between addresses
		m.SetWord(addr, data)
	}
}

func benchDenseMemoryUsage(b *testing.B, m *Memory) {
	data := Word(0x1234567890ABCDEF)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		addr := uint64(i) * 8 // Contiguous 8-byte allocations
		m.SetWord(addr, data)
	}
}

func benchSmallFrequentUpdates(b *testing.B, m *Memory) {
	data := Word(0x1234567890ABCDEF)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		addr := Word(rand.Intn(1000000)) & arch.AddressMask // Confined to a smaller range
		m.SetWord(addr, data)
	}
}

func benchMerkleProofGeneration(size int) func(b *testing.B, m *Memory) {
	return func(b *testing.B, m *Memory) {
		// Setup: allocate some memory
		for i := 0; i < size; i++ {
			m.SetWord(uint64(i)*8, Word(i))
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			addr := uint64(rand.Intn(size) * 8)
			_ = m.MerkleProof(addr)
		}
	}
}

func benchMerkleRootCalculation(size int) func(b *testing.B, m *Memory) {
	return func(b *testing.B, m *Memory) {
		// Setup: allocate some memory
		for i := 0; i < size; i++ {
			m.SetWord(uint64(i)*8, Word(i))
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = m.MerkleRoot()
		}
	}
}
