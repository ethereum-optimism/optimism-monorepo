package exec

import (
	"fmt"

	"github.com/ethereum-optimism/optimism/cannon/mipsevm/arch"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/memory"
)

type MemTracker interface {
	TrackMemAccess(addr Word)
}

type MemoryTrackerImpl struct {
	memory          *memory.Memory
	lastMemAccess   Word
	memProofEnabled bool
	// proof of first unique memory access
	memProof [memory.MemProofSize]byte
	// proof of second unique memory access
	memProof2 [memory.MemProofSize]byte
}

func NewMemoryTracker(memory *memory.Memory) *MemoryTrackerImpl {
	return &MemoryTrackerImpl{memory: memory}
}

func (m *MemoryTrackerImpl) TrackMemAccess(effAddr Word) {
	if m.memProofEnabled && m.lastMemAccess != effAddr {
		if m.lastMemAccess != ^Word(0) {
			panic(fmt.Errorf("unexpected different mem access at %08x, already have access at %08x buffered", effAddr, m.lastMemAccess))
		}
		m.lastMemAccess = effAddr
		m.memProof = m.memory.MerkleProof(effAddr)
	}
}

// TrackMemAccess2 creates a proof for a memory access following a call to TrackMemAccess
// This is used to generate proofs for contiguous memory accesses within the same step
func (m *MemoryTrackerImpl) TrackMemAccess2(effAddr Word) {
	if m.memProofEnabled && m.lastMemAccess+arch.WordSizeBytes != effAddr {
		panic(fmt.Errorf("unexpected disjointed mem access at %08x, last memory access is at %08x buffered", effAddr, m.lastMemAccess))
	}
	m.lastMemAccess = effAddr
	m.memProof2 = m.memory.MerkleProof(effAddr)
}

func (m *MemoryTrackerImpl) Reset(enableProof bool) {
	m.memProofEnabled = enableProof
	m.lastMemAccess = ^Word(0)
}

func (m *MemoryTrackerImpl) MemProof() [memory.MemProofSize]byte {
	return m.memProof
}

func (m *MemoryTrackerImpl) MemProof2() [memory.MemProofSize]byte {
	return m.memProof2
}

type NoopMemoryTracker struct{}

func (n *NoopMemoryTracker) TrackMemAccess(Word) {}
