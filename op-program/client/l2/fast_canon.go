package l2

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/beacon"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/triedb"
	"github.com/hashicorp/golang-lru/v2/simplelru"
)

// historicalCacheSize is the number of cached eip-2935 historical block lookups
// This covers 8 weeks worth of blocks on a 2 second block time.
// We keep a small cache size to ensure cache scans are fast.
const historicalCacheSize = 320

type FastCanonicalBlockHeaderOracle struct {
	head          *types.Header
	blockByHashFn BlockByHashFn
	config        *params.ChainConfig
	fallback      *CanonicalBlockHeaderOracle
	ctx           *chainContext
	db            ethdb.KeyValueStore
	cache         *simplelru.LRU[uint64, *types.Header]
}

func NewFastCanonicalBlockHeaderOracle(
	head *types.Header,
	blockByHashFn BlockByHashFn,
	chainCfg *params.ChainConfig,
	stateOracle StateOracle,
	kvdb KeyValueStore,
	fallback *CanonicalBlockHeaderOracle,
) *FastCanonicalBlockHeaderOracle {
	chainID := eth.ChainIDFromBig(chainCfg.ChainID)
	ctx := &chainContext{engine: beacon.New(nil)}
	db := NewOracleBackedDB(kvdb, stateOracle, chainID)
	cache, _ := simplelru.NewLRU[uint64, *types.Header](historicalCacheSize, nil)
	return &FastCanonicalBlockHeaderOracle{
		head:          head,
		blockByHashFn: blockByHashFn,
		config:        chainCfg,
		fallback:      fallback,
		ctx:           ctx,
		db:            db,
		cache:         cache,
	}
}

func (o *FastCanonicalBlockHeaderOracle) CurrentHeader() *types.Header {
	return o.head
}

func (o *FastCanonicalBlockHeaderOracle) GetHeaderByNumber(n uint64) *types.Header {
	if o.head.Number.Uint64() < n {
		return nil
	}
	if o.head.Number.Uint64() == n {
		return o.head
	}

	// scan the cache for a header that contains the requested block in its historical block window
	cover := uint64(math.MaxUint64)
	for _, number := range o.cache.Keys() {
		if number >= n && number < cover {
			cover = number
		}
	}
	h := o.head
	if cover != math.MaxUint64 {
		h, _ = o.cache.Get(cover)
	}
	if !o.config.IsIsthmus(h.Time) {
		return o.fallback.GetHeaderByNumber(n)
	}

	for h.Number.Uint64() > n {
		headNumber := h.Number.Uint64()
		var currEarliestHistory uint64
		if params.HistoryServeWindow-1 < headNumber {
			currEarliestHistory = headNumber - (params.HistoryServeWindow - 1)
		}
		if currEarliestHistory <= n {
			block := o.getHistoricalBlockHash(h, n)
			if block == nil {
				return o.fallback.GetHeaderByNumber(n)
			}
			return block.Header()
		}
		block := o.getHistoricalBlockHash(h, currEarliestHistory)
		if block == nil {
			return o.fallback.GetHeaderByNumber(n)
		}
		h = block.Header()
		o.cache.Add(h.Number.Uint64(), h)
	}
	return h
}

func (o *FastCanonicalBlockHeaderOracle) getHistoricalBlockHash(head *types.Header, n uint64) *types.Block {
	statedb, err := state.New(head.Root, state.NewDatabase(triedb.NewDatabase(rawdb.NewDatabase(o.db), nil), nil))
	if err != nil {
		panic(fmt.Errorf("failed to get state at %v: %w", head.Hash(), err))
	}
	// for safety. But it shouldn't be required since we only read from state
	statedb.MakeSinglethreaded()

	context := core.NewEVMBlockContext(head, o.ctx, nil, o.config, statedb)
	vmenv := vm.NewEVM(context, statedb, o.config, vm.Config{})
	var caller vm.AccountRef // can be anything as long as it's not the system contract
	gas := uint64(1000000)
	var input [32]byte
	binary.BigEndian.PutUint64(input[24:], n)
	ret, _, err := vmenv.StaticCall(caller, params.HistoryStorageAddress, input[:], gas)
	if err != nil {
		panic(fmt.Errorf("failed to get history block hash: %w", err))
	}
	if len(ret) != 32 {
		panic(fmt.Errorf("invalid history storage result. got %d bytes, expected %d bytes", len(ret), common.HashLength))
	}
	hash := common.Hash(ret)
	if hash == (common.Hash{}) {
		// we're near eip-2935 activation so the history ringbuffer isn't filled up yet
		return nil
	}
	header := o.blockByHashFn(hash)
	if header == nil {
		panic(fmt.Errorf("failed to get history block header for %v", n))
	}
	return header
}

func (o *FastCanonicalBlockHeaderOracle) SetCanonical(head *types.Header) common.Hash {
	o.head = head
	o.fallback.SetCanonical(head)
	o.cache.Purge()
	return head.Hash()
}

type chainContext struct {
	engine consensus.Engine
}

func (c *chainContext) Engine() consensus.Engine {
	return c.engine
}

func (c *chainContext) GetHeader(hash common.Hash, number uint64) *types.Header {
	// The EVM should never call this method during eip-2935 historical block retrieval
	panic("unexpected call to GetHeader")
}
