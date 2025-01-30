package common

import (
	"bytes"

	"github.com/ethereum-optimism/optimism/op-program/client/l2"
	"github.com/ethereum-optimism/optimism/op-program/host/kvstore"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/ethdb"
)

type l2KeyValueStore struct {
	kv kvstore.KV
}

var _ l2.KeyValueStore = (*l2KeyValueStore)(nil)

// NewL2KeyValueStore creates a l2.KeyValueStore compatible database that's backed by a [kvstore.KV]
func NewL2KeyValueStore(kv kvstore.KV) *l2KeyValueStore {
	return &l2KeyValueStore{kv: kv}
}

var codePrefixedKeyLength = common.HashLength + len(rawdb.CodePrefix)

func unwrapKey(key []byte) []byte {
	if len(key) == codePrefixedKeyLength && bytes.HasPrefix(key, rawdb.CodePrefix) {
		return key[len(rawdb.CodePrefix):]
	}
	return key
}

func (db *l2KeyValueStore) Get(key []byte) ([]byte, error) {
	key = unwrapKey(key)
	if len(key) != common.HashLength {
		return nil, l2.ErrInvalidKeyLength
	}
	return db.kv.Get(common.Hash(key))
}

func (db *l2KeyValueStore) Has(key []byte) (bool, error) {
	key = unwrapKey(key)
	if len(key) != common.HashLength {
		return false, l2.ErrInvalidKeyLength
	}
	_, err := db.kv.Get(common.Hash(key))
	switch err {
	case kvstore.ErrNotFound:
		return false, nil
	case nil:
		return true, nil
	default:
		return false, err
	}
}

func (db *l2KeyValueStore) Put(key []byte, value []byte) error {
	key = unwrapKey(key)
	// For statedb operations, we only expect code and preimage keys of hash length
	if len(key) != common.HashLength {
		return l2.ErrInvalidKeyLength
	}
	return db.kv.Put(common.Hash(key), value)
}

func (db *l2KeyValueStore) NewBatch() ethdb.Batch {
	return &batch{db: db}
}

func (db *l2KeyValueStore) NewBatchWithSize(size int) ethdb.Batch {
	return &batch{db: db}
}

// batch is similar to memorydb.batch, but adapted for kvstore.KV
type batch struct {
	db     *l2KeyValueStore
	writes []keyvalue
	size   int
}

var _ ethdb.Batch = (*batch)(nil)

type keyvalue struct {
	key   []byte
	value []byte
}

func (b *batch) Put(key []byte, value []byte) error {
	b.writes = append(b.writes, keyvalue{common.CopyBytes(key), common.CopyBytes(value)})
	b.size += len(key) + len(value)
	return nil
}

func (b *batch) Delete(key []byte) error {
	// ignore deletes
	return nil
}

func (b *batch) ValueSize() int {
	return b.size
}

func (b *batch) Write() error {
	for _, keyvalue := range b.writes {
		if err := b.db.kv.Put(common.Hash(keyvalue.key), keyvalue.value); err != nil {
			return err
		}
	}
	return nil
}

func (b *batch) Reset() {
	b.writes = b.writes[:0]
	b.size = 0
}

func (b *batch) Replay(w ethdb.KeyValueWriter) error {
	for _, keyvalue := range b.writes {
		if err := w.Put(keyvalue.key, keyvalue.value); err != nil {
			return err
		}
	}
	return nil
}
