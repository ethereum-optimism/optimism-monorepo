package memory

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math/bits"
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
	// generalized index -> merkle root or nil if invalidated
	nodes map[uint64]*[32]byte

	// pageIndex -> cached page
	pages map[Word]*CachedPage

	// Note: since we don't de-alloc pages, we don't do ref-counting.
	// Once a page exists, it doesn't leave memory

	// two caches: we often read instructions from one page, and do memory things with another page.
	// this prevents map lookups each instruction
	lastPageKeys [2]Word
	lastPage     [2]*CachedPage
}

func NewMemory() *Memory {
	return &Memory{
		nodes:        make(map[uint64]*[32]byte),
		pages:        make(map[Word]*CachedPage),
		lastPageKeys: [2]Word{^Word(0), ^Word(0)}, // default to invalid keys, to not match any pages
	}
}

func (m *Memory) PageCount() int {
	return len(m.pages)
}

func (m *Memory) ForEachPage(fn func(pageIndex Word, page *Page) error) error {
	for pageIndex, cachedPage := range m.pages {
		if err := fn(pageIndex, cachedPage.Data); err != nil {
			return err
		}
	}
	return nil
}

func (m *Memory) invalidate(addr Word) {
	// addr must be aligned
	if addr&arch.ExtMask != 0 {
		panic(fmt.Errorf("unaligned memory access: %x", addr))
	}

	// find page, and invalidate addr within it
	if p, ok := m.pageLookup(addr >> PageAddrSize); ok {
		prevValid := p.Ok[1]
		p.invalidate(addr & PageAddrMask)
		if !prevValid { // if the page was already invalid before, then nodes to mem-root will also still be.
			return
		}
	} else { // no page? nothing to invalidate
		return
	}

	// find the gindex of the first page covering the address: i.e. ((1 << WordSize) | addr) >> PageAddrSize
	// Avoid 64-bit overflow by distributing the right shift across the OR.
	gindex := (uint64(1) << (WordSize - PageAddrSize)) | uint64(addr>>PageAddrSize)

	for gindex > 0 {
		m.nodes[gindex] = nil
		gindex >>= 1
	}
}

func (m *Memory) MerkleizeSubtree(gindex uint64) [32]byte {
	l := uint64(bits.Len64(gindex))
	if l > MemProofLeafCount {
		panic("gindex too deep")
	}
	if l > PageKeySize {
		depthIntoPage := l - 1 - PageKeySize
		pageIndex := (gindex >> depthIntoPage) & PageKeyMask
		if p, ok := m.pages[Word(pageIndex)]; ok {
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

func (m *Memory) MerkleProof(addr Word) (out [MemProofSize]byte) {
	proof := m.traverseBranch(1, addr, 0)
	// encode the proof
	for i := 0; i < MemProofLeafCount; i++ {
		copy(out[i*32:(i+1)*32], proof[i][:])
	}
	return out
}

func (m *Memory) traverseBranch(parent uint64, addr Word, depth uint8) (proof [][32]byte) {
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

func (m *Memory) MerkleRoot() [32]byte {
	return m.MerkleizeSubtree(1)
}

func (m *Memory) pageLookup(pageIndex Word) (*CachedPage, bool) {
	// hit caches
	if pageIndex == m.lastPageKeys[0] {
		return m.lastPage[0], true
	}
	if pageIndex == m.lastPageKeys[1] {
		return m.lastPage[1], true
	}
	p, ok := m.pages[pageIndex]

	// only cache existing pages.
	if ok {
		m.lastPageKeys[1] = m.lastPageKeys[0]
		m.lastPage[1] = m.lastPage[0]
		m.lastPageKeys[0] = pageIndex
		m.lastPage[0] = p
	}

	return p, ok
}

// SetWord stores [arch.Word] sized values at the specified address
func (m *Memory) SetWord(addr Word, v Word) {
	// addr must be aligned to WordSizeBytes bytes
	if addr&arch.ExtMask != 0 {
		panic(fmt.Errorf("unaligned memory access: %x", addr))
	}

	pageIndex := addr >> PageAddrSize
	pageAddr := addr & PageAddrMask
	p, ok := m.pageLookup(pageIndex)
	if !ok {
		// allocate the page if we have not already.
		// Go may mmap relatively large ranges, but we only allocate the pages just in time.
		p = m.AllocPage(pageIndex)
	} else {
		m.invalidate(addr) // invalidate this branch of memory, now that the value changed
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
	p, ok := m.pageLookup(addr >> PageAddrSize)
	if !ok {
		return 0
	}
	pageAddr := addr & PageAddrMask
	return arch.ByteOrderWord.Word(p.Data[pageAddr : pageAddr+arch.WordSizeBytes])
}

func (m *Memory) AllocPage(pageIndex Word) *CachedPage {
	p := &CachedPage{Data: new(Page)}
	m.pages[pageIndex] = p
	// make nodes to root
	k := (1 << PageKeySize) | uint64(pageIndex)
	for k > 0 {
		m.nodes[k] = nil
		k >>= 1
	}
	return p
}

type pageEntry struct {
	Index Word  `json:"index"`
	Data  *Page `json:"data"`
}

func (m *Memory) MarshalJSON() ([]byte, error) { // nosemgrep
	pages := make([]pageEntry, 0, len(m.pages))
	for k, p := range m.pages {
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
	m.nodes = make(map[uint64]*[32]byte)
	m.pages = make(map[Word]*CachedPage)
	m.lastPageKeys = [2]Word{^Word(0), ^Word(0)}
	m.lastPage = [2]*CachedPage{nil, nil}
	for i, p := range pages {
		if _, ok := m.pages[p.Index]; ok {
			return fmt.Errorf("cannot load duplicate page, entry %d, page index %d", i, p.Index)
		}
		m.AllocPage(p.Index).Data = p.Data
	}
	return nil
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

		p, ok := m.pageLookup(pageIndex)
		if !ok {
			p = m.AllocPage(pageIndex)
		}
		p.InvalidateFull()
		copy(p.Data[pageAddr:], chunk[:n])
		addr += Word(n)
	}
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
	indexes := maps.Keys(m.pages)
	// iterate sorted map keys for consistent serialization
	slices.Sort(indexes)
	for _, pageIndex := range indexes {
		page := m.pages[pageIndex]
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

func (m *Memory) Copy() *Memory {
	out := NewMemory()
	out.nodes = make(map[uint64]*[32]byte)
	out.pages = make(map[Word]*CachedPage)
	out.lastPageKeys = [2]Word{^Word(0), ^Word(0)}
	out.lastPage = [2]*CachedPage{nil, nil}
	for k, page := range m.pages {
		data := new(Page)
		*data = *page.Data
		out.AllocPage(k).Data = data
	}
	return out
}

type memReader struct {
	m     *Memory
	addr  Word
	count Word
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
	p, ok := r.m.pageLookup(pageIndex)
	if ok {
		n = copy(dest, p.Data[start:end])
	} else {
		n = copy(dest, make([]byte, end-start)) // default to zeroes
	}
	r.addr += Word(n)
	r.count -= Word(n)
	return n, nil
}

func (m *Memory) ReadMemoryRange(addr Word, count Word) io.Reader {
	return &memReader{m: m, addr: addr, count: count}
}

func (m *Memory) UsageRaw() uint64 {
	return uint64(len(m.pages)) * PageSize
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
