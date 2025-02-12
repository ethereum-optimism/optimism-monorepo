package memory

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"sort"

	"github.com/ethereum-optimism/optimism/cannon/mipsevm/arch"
	"github.com/ethereum/go-ethereum/crypto"
	"golang.org/x/exp/maps"
)

// Note: 2**12 = 4 KiB, the min phys page size in the Go runtime.
const (
	WordSize          = arch.WordSize
	PageAddrSize      = arch.PageAddrSize
	PageKeySize       = arch.PageKeySize
	PageSize          = 1 << PageAddrSize
	PageAddrMask      = PageSize - 1
	MaxPageCount      = 1 << PageKeySize
	PageKeyMask       = MaxPageCount - 1
	MemProofLeafCount = arch.MemProofLeafCount
	MemProofSize      = arch.MemProofSize
)

type Word = arch.Word

func HashPair(left, right [32]byte) [32]byte {
	out := crypto.Keccak256Hash(left[:], right[:])
	//fmt.Printf("0x%x 0x%x -> 0x%x\n", left, right, out)
	return out
}

var zeroHashes = func() [256][32]byte {
	// empty parts of the tree are all zero. Precompute the hash of each full-zero range sub-tree level.
	var out [256][32]byte
	for i := 1; i < 256; i++ {
		out[i] = HashPair(out[i-1], out[i-1])
	}
	return out
}()

type Memory struct {
	merkleIndex PageIndex
	// Note: since we don't de-alloc Pages, we don't do ref-counting.
	// Once a page exists, it doesn't leave memory
	pageTable map[Word]*CachedPage

	// two caches: we often read instructions from one page, and do memory things with another page.
	// this prevents map lookups each instruction
	lastPageKeys [2]Word
	lastPage     [2]*CachedPage
}

type PageIndex interface {
	MerkleRoot() [32]byte
	AddPage(pageIndex Word)
	MerkleProof(addr Word) [MemProofSize]byte
	MerkleizeSubtree(gindex uint64) [32]byte
	Invalidate(addr Word)

	New() PageIndex
	setPageBacking(pages map[Word]*CachedPage)
}

func (m *Memory) MerkleRoot() [32]byte {
	return m.MerkleizeSubtree(1)
}

func (m *Memory) MerkleProof(addr Word) [MemProofSize]byte {
	return m.merkleIndex.MerkleProof(addr)
}

func (m *Memory) PageCount() int {
	return len(m.pageTable)
}

func (m *Memory) ForEachPage(fn func(pageIndex Word, page *Page) error) error {
	for pageIndex, cachedPage := range m.pageTable {
		if err := fn(pageIndex, cachedPage.Data); err != nil {
			return err
		}
	}
	return nil
}

func (m *Memory) MerkleizeSubtree(gindex uint64) [32]byte {
	return m.merkleIndex.MerkleizeSubtree(gindex)
}

func (m *Memory) PageLookup(pageIndex Word) (*CachedPage, bool) {
	// hit caches
	if pageIndex == m.lastPageKeys[0] {
		return m.lastPage[0], true
	}
	if pageIndex == m.lastPageKeys[1] {
		return m.lastPage[1], true
	}
	p, ok := m.pageTable[pageIndex]

	// only cache existing pages.
	if ok {
		m.lastPageKeys[1] = m.lastPageKeys[0]
		m.lastPage[1] = m.lastPage[0]
		m.lastPageKeys[0] = pageIndex
		m.lastPage[0] = p
	}

	return p, ok
}

func (m *Memory) Invalidate(addr Word) {
	// addr must be aligned
	if addr&arch.ExtMask != 0 {
		panic(fmt.Errorf("unaligned memory access: %x", addr))
	}

	// find page, and invalidate addr within it
	if p, ok := m.PageLookup(addr >> PageAddrSize); ok {
		prevValid := p.Ok[1]
		p.invalidate(addr & PageAddrMask)
		if !prevValid { // if the page was already invalid before, then nodes to mem-root will also still be.
			return
		}
	} else { // no page? nothing to invalidate
		return
	}
	// invalidate the path to the page in the table
	m.merkleIndex.Invalidate(addr)
}

func (m *Memory) SetMemoryRange(addr Word, r io.Reader) error {
	for {
		pageIndex := addr >> PageAddrSize
		pageAddr := addr & PageAddrMask
		readLen := PageSize - pageAddr
		chunk := make([]byte, readLen)
		n, err := r.Read(chunk)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		p, ok := m.PageLookup(pageIndex)
		if !ok {
			p = m.AllocPage(pageIndex)
		}
		p.InvalidateFull()
		copy(p.Data[pageAddr:], chunk[:n])
		addr += Word(n)
	}
}

// SetWord stores [arch.Word] sized values at the specified address
func (m *Memory) SetWord(addr Word, v Word) {
	// addr must be aligned to WordSizeBytes bytes
	if addr&arch.ExtMask != 0 {
		panic(fmt.Errorf("unaligned memory access: %x", addr))
	}

	pageIndex := addr >> PageAddrSize
	pageAddr := addr & PageAddrMask
	p, ok := m.PageLookup(pageIndex)
	if !ok {
		// allocate the page if we have not already.
		// Go may mmap relatively large ranges, but we only allocate the pages just in time.
		p = m.AllocPage(pageIndex)
	} else {
		m.Invalidate(addr) // invalidate this branch of memory, now that the value changed
	}
	arch.ByteOrderWord.PutWord(p.Data[pageAddr:pageAddr+arch.WordSizeBytes], v)
}

// GetWord reads the maximum sized value, [arch.Word], located at the specified address.
// Note: Also referred to by the MIPS64 specification as a "double-word" memory access.
func (m *Memory) GetWord(addr Word) Word {
	// addr must be word aligned
	if addr&arch.ExtMask != 0 {
		panic(fmt.Errorf("unaligned memory access: %x", addr))
	}
	pageIndex := addr >> PageAddrSize
	p, ok := m.PageLookup(pageIndex)
	if !ok {
		return 0
	}
	pageAddr := addr & PageAddrMask
	return arch.ByteOrderWord.Word(p.Data[pageAddr : pageAddr+arch.WordSizeBytes])
}

func (m *Memory) AllocPage(pageIndex Word) *CachedPage {
	p := &CachedPage{Data: new(Page)}
	m.pageTable[pageIndex] = p
	m.merkleIndex.AddPage(pageIndex)
	return p
}

type memReader struct {
	m     *Memory
	addr  Word
	count Word
}

func (m *Memory) ReadMemoryRange(addr Word, count Word) io.Reader {
	return &memReader{m: m, addr: addr, count: count}
}
func (r *memReader) Read(dest []byte) (n int, err error) {
	if r.count == 0 {
		return 0, io.EOF
	}

	// Keep iterating over memory until we have all our data.
	// It may wrap around the address range, and may not be aligned
	endAddr := r.addr + r.count

	pageIndex := r.addr >> PageAddrSize
	start := r.addr & PageAddrMask
	end := Word(PageSize)

	if pageIndex == (endAddr >> PageAddrSize) {
		end = endAddr & PageAddrMask
	}
	p, ok := r.m.PageLookup(pageIndex)
	if ok {
		n = copy(dest, p.Data[start:end])
	} else {
		n = copy(dest, make([]byte, end-start)) // default to zeroes
	}
	r.addr += Word(n)
	r.count -= Word(n)
	return n, nil
}

func (m *Memory) UsageRaw() uint64 {
	return uint64(len(m.pageTable)) * PageSize
}

func (m *Memory) Usage() string {
	total := m.UsageRaw()
	const unit = 1024
	if total < unit {
		return fmt.Sprintf("%d B", total)
	}
	div, exp := uint64(unit), 0
	for n := total / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	// KiB, MiB, GiB, TiB, ...
	return fmt.Sprintf("%.1f %ciB", float64(total)/float64(div), "KMGTPE"[exp])
}

func (m *Memory) Copy() *Memory {
	pages := make(map[Word]*CachedPage)
	table := m.merkleIndex.New()
	table.setPageBacking(pages)
	out := &Memory{
		merkleIndex:  table,
		pageTable:    pages,
		lastPageKeys: [2]Word{^Word(0), ^Word(0)}, // default to invalid keys, to not match any pages
		lastPage:     [2]*CachedPage{nil, nil},
	}

	for k, page := range m.pageTable {
		data := new(Page)
		*data = *page.Data
		out.AllocPage(k).Data = data
	}
	return out
}

// Serialize writes the memory in a simple binary format which can be read again using Deserialize
// The format is a simple concatenation of fields, with prefixed item count for repeating items and using big endian
// encoding for numbers.
//
// len(PageCount)    Word
// For each page (order is arbitrary):
//
//	page index          Word
//	page Data           [PageSize]byte
func (m *Memory) Serialize(out io.Writer) error {
	if err := binary.Write(out, binary.BigEndian, Word(m.PageCount())); err != nil {
		return err
	}
	indexes := maps.Keys(m.pageTable)
	// iterate sorted map keys for consistent serialization
	slices.Sort(indexes)
	for _, pageIndex := range indexes {
		page := m.pageTable[pageIndex]
		if err := binary.Write(out, binary.BigEndian, pageIndex); err != nil {
			return err
		}
		if _, err := out.Write(page.Data[:]); err != nil {
			return err
		}
	}
	return nil
}

func (m *Memory) Deserialize(in io.Reader) error {
	var pageCount Word
	if err := binary.Read(in, binary.BigEndian, &pageCount); err != nil {
		return err
	}
	for i := Word(0); i < pageCount; i++ {
		var pageIndex Word
		if err := binary.Read(in, binary.BigEndian, &pageIndex); err != nil {
			return err
		}
		page := m.AllocPage(pageIndex)
		if _, err := io.ReadFull(in, page.Data[:]); err != nil {
			return err
		}
	}
	return nil
}

type pageEntry struct {
	Index Word  `json:"index"`
	Data  *Page `json:"data"`
}

func (m *Memory) MarshalJSON() ([]byte, error) { // nosemgrep
	pages := make([]pageEntry, 0, len(m.pageTable))
	for k, p := range m.pageTable {
		pages = append(pages, pageEntry{
			Index: k,
			Data:  p.Data,
		})
	}
	sort.Slice(pages, func(i, j int) bool {
		return pages[i].Index < pages[j].Index
	})
	return json.Marshal(pages)
}

func (m *Memory) UnmarshalJSON(data []byte) error {
	var pages []pageEntry
	if err := json.Unmarshal(data, &pages); err != nil {
		return err
	}
	for i, p := range pages {
		if _, ok := m.pageTable[p.Index]; ok {
			return fmt.Errorf("cannot load duplicate page, entry %d, page index %d", i, p.Index)
		}
		m.AllocPage(p.Index).Data = p.Data
	}
	return nil
}
