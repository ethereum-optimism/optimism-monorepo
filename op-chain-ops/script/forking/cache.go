package forking

import (
	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/holiman/uint256"

	"github.com/ethereum/go-ethereum/common"
)

type storageKey struct {
	Addr common.Address
	Slot common.Hash
}

// CachedSource wraps a ForkSource, and caches the retrieved data for faster repeat-queries.
// The ForkSource should be immutable (as per the StateRoot value).
// All cache data accumulates in-memory in LRU collections per data type.
type CachedSource struct {
	stateRoot common.Hash
	src       ForkSource

	nonces   *lru.Cache[common.Address, uint64]
	balances *lru.Cache[common.Address, *uint256.Int]
	storage  *lru.Cache[storageKey, common.Hash]
	code     *lru.Cache[common.Address, []byte]
}

var _ ForkSource = (*CachedSource)(nil)

func mustNewLRU[K comparable, V any](size int) *lru.Cache[K, V] {
	out, err := lru.New[K, V](size)
	if err != nil {
		panic(err) // bad size parameter may produce an error
	}
	return out
}

func Cache(src ForkSource) *CachedSource {
	return &CachedSource{
		stateRoot: src.StateRoot(),
		src:       src,
		nonces:    mustNewLRU[common.Address, uint64](1000),
		balances:  mustNewLRU[common.Address, *uint256.Int](1000),
		storage:   mustNewLRU[storageKey, common.Hash](1000),
		code:      mustNewLRU[common.Address, []byte](100),
	}
}

func (c *CachedSource) URLOrAlias() string {
	return c.src.URLOrAlias()
}

func (c *CachedSource) StateRoot() common.Hash {
	return c.stateRoot
}

func (c *CachedSource) Nonce(addr common.Address) (uint64, error) {
	if v, ok := c.nonces.Get(addr); ok {
		return v, nil
	}
	v, err := c.src.Nonce(addr)
	if err != nil {
		return 0, err
	}
	c.nonces.Add(addr, v)
	return v, nil
}

func (c *CachedSource) Balance(addr common.Address) (*uint256.Int, error) {
	if v, ok := c.balances.Get(addr); ok {
		return v.Clone(), nil
	}
	v, err := c.src.Balance(addr)
	if err != nil {
		return nil, err
	}
	c.balances.Add(addr, v)
	return v.Clone(), nil
}

func (c *CachedSource) StorageAt(addr common.Address, key common.Hash) (common.Hash, error) {
	if v, ok := c.storage.Get(storageKey{Addr: addr, Slot: key}); ok {
		return v, nil
	}
	v, err := c.src.StorageAt(addr, key)
	if err != nil {
		return common.Hash{}, err
	}
	c.storage.Add(storageKey{Addr: addr, Slot: key}, v)
	return v, nil
}

func (c *CachedSource) Code(addr common.Address) ([]byte, error) {
	if v, ok := c.code.Get(addr); ok {
		return v, nil
	}
	v, err := c.src.Code(addr)
	if err != nil {
		return nil, err
	}
	c.code.Add(addr, v)
	return v, nil
}
