package arch

// This file contains stuff common to both arch32 and arch64

const (
	IsMips32      = WordSize == 32
	WordSizeBytes = WordSize >> 3
	PageAddrSize  = 12
	PageKeySize   = WordSize - PageAddrSize

	MemProofLeafCount = WordSize - 4
	MemProofSize      = MemProofLeafCount * 32
)
