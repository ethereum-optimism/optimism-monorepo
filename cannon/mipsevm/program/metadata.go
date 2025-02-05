package program

import (
	"debug/elf"
	"fmt"
	"sort"

	"github.com/ethereum-optimism/optimism/cannon/mipsevm"
)

type Symbol struct {
	Name  string `json:"name"`
	Start Word   `json:"start"`
	Size  Word   `json:"size"`
}

type Metadata struct {
	Symbols []Symbol `json:"symbols"`
}

var _ mipsevm.Metadata = (*Metadata)(nil)

func MakeMetadata(elfProgram *elf.File) (*Metadata, error) {
	syms, err := elfProgram.Symbols()
	if err != nil {
		return nil, fmt.Errorf("failed to load symbols table: %w", err)
	}
	// Make sure the table is sorted, Go outputs mostly sorted data, except some internal functions
	sort.Slice(syms, func(i, j int) bool {
		return syms[i].Value < syms[j].Value
	})
	out := &Metadata{Symbols: make([]Symbol, len(syms))}
	for i, s := range syms {
		out.Symbols[i] = Symbol{Name: s.Name, Start: Word(s.Value), Size: Word(s.Size)}
	}
	return out, nil
}

func (m *Metadata) LookupSymbol(addr Word) string {
	if len(m.Symbols) == 0 {
		return "!unknown"
	}
	// find first symbol with higher start. Or n if no such symbol exists
	i := sort.Search(len(m.Symbols), func(i int) bool {
		return m.Symbols[i].Start > addr
	})
	if i == 0 {
		return "!start"
	}
	out := &m.Symbols[i-1]
	if out.Start+out.Size < addr { // addr may be pointing to a gap between symbols
		return "!gap"
	}
	return out.Name
}

func (m *Metadata) CreateSymbolMatcher(name string) mipsevm.SymbolMatcher {
	for _, s := range m.Symbols {
		if s.Name == name {
			start := s.Start
			end := s.Start + s.Size
			return func(addr Word) bool {
				return addr >= start && addr < end
			}
		}
	}
	return func(addr Word) bool {
		return false
	}
}
