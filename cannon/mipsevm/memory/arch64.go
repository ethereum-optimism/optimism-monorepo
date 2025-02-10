//go:build cannon64
// +build cannon64

package memory

func NewMemory() *Memory {
	return NewBinaryTreeMemory()
}

func NewTrieMemory() *Memory {
	pages := make(map[Word]*CachedPage)
	index := NewTrieIndex()
	index.setPageBacking(pages)
	return &Memory{
		pageTable:    pages,
		merkleIndex:  index,
		lastPageKeys: [2]Word{^Word(0), ^Word(0)}, // default to invalid keys, to not match any pages
	}
}
