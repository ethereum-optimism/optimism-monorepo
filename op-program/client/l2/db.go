package l2

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/ethdb"
)

var codePrefixedKeyLength = common.HashLength + len(rawdb.CodePrefix)

var ErrInvalidKeyLength = errors.New("pre-images must be identified by 32-byte hash keys")

// KeyValueStore is a subset of the ethdb.KeyValueStore interface that's required for block processing.
type KeyValueStore interface {
	ethdb.KeyValueReader
	ethdb.Batcher
	// Put inserts the given value into the key-value data store.
	Put(key []byte, value []byte) error
}

type OracleKeyValueStore struct {
	db      KeyValueStore
	oracle  StateOracle
	chainID eth.ChainID
}

func NewOracleBackedDB(kv KeyValueStore, oracle StateOracle, chainID eth.ChainID) *OracleKeyValueStore {
	return &OracleKeyValueStore{
		db:      kv,
		oracle:  oracle,
		chainID: chainID,
	}
}

func (o *OracleKeyValueStore) Get(key []byte) ([]byte, error) {
	has, err := o.db.Has(key)
	if err != nil {
		return nil, fmt.Errorf("checking in-memory db: %w", err)
	}
	if has {
		return o.db.Get(key)
	}

	if len(key) == codePrefixedKeyLength && bytes.HasPrefix(key, rawdb.CodePrefix) {
		key = key[len(rawdb.CodePrefix):]
		return o.oracle.CodeByHash(*(*[common.HashLength]byte)(key), o.chainID), nil
	}
	if len(key) != common.HashLength {
		return nil, ErrInvalidKeyLength
	}
	return o.oracle.NodeByHash(*(*[common.HashLength]byte)(key), o.chainID), nil
}

func (o *OracleKeyValueStore) NewBatch() ethdb.Batch {
	return o.db.NewBatch()
}

func (o *OracleKeyValueStore) NewBatchWithSize(size int) ethdb.Batch {
	return o.db.NewBatchWithSize(size)
}

func (o *OracleKeyValueStore) Put(key []byte, value []byte) error {
	return o.db.Put(key, value)
}

func (o *OracleKeyValueStore) Close() error {
	return nil
}

// Remaining methods are unused when accessing the state for block processing so leaving unimplemented.

func (o *OracleKeyValueStore) Has(key []byte) (bool, error) {
	panic("not supported")
}

func (o *OracleKeyValueStore) Delete(key []byte) error {
	panic("not supported")
}

func (o *OracleKeyValueStore) DeleteRange(start, end []byte) error {
	panic("not supported")
}

func (o *OracleKeyValueStore) Stat() (string, error) {
	panic("not supported")
}

func (o *OracleKeyValueStore) NewIterator(prefix []byte, start []byte) ethdb.Iterator {
	panic("not supported")
}

func (o *OracleKeyValueStore) Compact(start []byte, limit []byte) error {
	panic("not supported")
}
