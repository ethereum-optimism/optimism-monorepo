package memory

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/ethereum/go-ethereum/crypto"
)

var zlibWriterPool = sync.Pool{
	New: func() any {
		var buf bytes.Buffer
		return zlib.NewWriter(&buf)
	},
}

type Page [PageSize]byte

func (p *Page) MarshalJSON() ([]byte, error) { // nosemgrep
	var out bytes.Buffer
	w := zlibWriterPool.Get().(*zlib.Writer)
	defer zlibWriterPool.Put(w)
	w.Reset(&out)
	if _, err := w.Write(p[:]); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return json.Marshal(out.Bytes())
}

func (p *Page) UnmarshalJSON(dat []byte) error {
	// Strip off the `"` characters at the start & end.
	dat = dat[1 : len(dat)-1]
	// Decode b64 then decompress
	r, err := zlib.NewReader(base64.NewDecoder(base64.StdEncoding, bytes.NewReader(dat)))
	if err != nil {
		return err
	}
	defer r.Close()
	if n, err := r.Read(p[:]); n != PageSize {
		return fmt.Errorf("epxeted %d bytes, but got %d", PageSize, n)
	} else if err == io.EOF {
		return nil
	} else {
		return err
	}
}

func (p *Page) UnmarshalText(dat []byte) error {
	if len(dat) != PageSize*2 {
		return fmt.Errorf("expected %d hex chars, but got %d", PageSize*2, len(dat))
	}
	_, err := hex.Decode(p[:], dat)
	return err
}

type CachedPage struct {
	Data *Page
	// intermediate nodes only
	Cache [PageSize / 32][32]byte
	// true if the intermediate node is valid
	Ok [PageSize / 32]bool
}

func (p *CachedPage) invalidate(pageAddr Word) {
	if pageAddr >= PageSize {
		panic("invalid page addr")
	}
	k := (1 << PageAddrSize) | pageAddr
	// first cache layer caches nodes that has two 32 byte leaf nodes.
	k >>= 5 + 1
	for k > 0 {
		p.Ok[k] = false
		k >>= 1
	}
}

func (p *CachedPage) InvalidateFull() {
	p.Ok = [PageSize / 32]bool{} // reset everything to false
}

func (p *CachedPage) MerkleRoot() [32]byte {
	// hash the bottom layer
	for i := uint64(0); i < PageSize; i += 64 {
		j := PageSize/32/2 + i/64
		if p.Ok[j] {
			continue
		}
		p.Cache[j] = crypto.Keccak256Hash(p.Data[i : i+64])
		//fmt.Printf("0x%x 0x%x -> 0x%x\n", p.Data[i:i+32], p.Data[i+32:i+64], p.Cache[j])
		p.Ok[j] = true
	}

	// hash the cache layers
	for i := PageSize/32 - 2; i > 0; i -= 2 {
		j := i >> 1
		if p.Ok[j] {
			continue
		}
		p.Cache[j] = HashPair(p.Cache[i], p.Cache[i+1])
		p.Ok[j] = true
	}

	return p.Cache[1]
}

func (p *CachedPage) MerkleizeSubtree(gindex uint64) [32]byte {
	_ = p.MerkleRoot() // fill cache
	if gindex >= PageSize/32 {
		if gindex >= PageSize/32*2 {
			panic("gindex too deep")
		}
		// it's pointing to a bottom node
		nodeIndex := gindex & (PageAddrMask >> 5)
		return *(*[32]byte)(p.Data[nodeIndex*32 : nodeIndex*32+32])
	}
	return p.Cache[gindex]
}
