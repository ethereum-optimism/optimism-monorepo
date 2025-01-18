package mpt

import "github.com/ethereum/go-ethereum/ethdb"

type Hooks struct {
	Get    func(key []byte) []byte
	Put    func(key []byte, value []byte)
	Delete func(key []byte)
}

// DB implements the ethdb.Database to back the StateDB of Geth.
type DB struct {
	db Hooks
}

func (p *DB) Has(key []byte) (bool, error) {
	panic("not supported")
}

func (p *DB) Get(key []byte) ([]byte, error) {
	return p.db.Get(key), nil
}

func (p *DB) Put(key []byte, value []byte) error {
	p.db.Put(key, value)
	return nil
}

func (p DB) Delete(key []byte) error {
	p.db.Delete(key)
	return nil
}

func (p *DB) DeleteRange(start, end []byte) error {
	panic("not supported")
}

func (p DB) Stat() (string, error) {
	panic("not supported")
}

func (p DB) NewBatch() ethdb.Batch {
	panic("not supported")
}

func (p DB) NewBatchWithSize(size int) ethdb.Batch {
	panic("not supported")
}

func (p DB) NewIterator(prefix []byte, start []byte) ethdb.Iterator {
	panic("not supported")
}

func (p DB) Compact(start []byte, limit []byte) error {
	return nil // no-op
}

func (p DB) Close() error {
	return nil
}

// We implement the full ethdb.Database bloat because the StateDB takes this full interface,
// even though it only uses the KeyValue subset.

func (p *DB) HasAncient(kind string, number uint64) (bool, error) {
	panic("not supported")
}

func (p *DB) Ancient(kind string, number uint64) ([]byte, error) {
	panic("not supported")
}

func (p *DB) AncientRange(kind string, start, count, maxBytes uint64) ([][]byte, error) {
	panic("not supported")
}

func (p *DB) Ancients() (uint64, error) {
	panic("not supported")
}

func (p *DB) Tail() (uint64, error) {
	panic("not supported")
}

func (p *DB) AncientSize(kind string) (uint64, error) {
	panic("not supported")
}

func (p *DB) ReadAncients(fn func(ethdb.AncientReaderOp) error) (err error) {
	panic("not supported")
}

func (p *DB) ModifyAncients(f func(ethdb.AncientWriteOp) error) (int64, error) {
	panic("not supported")
}

func (p *DB) TruncateHead(n uint64) (uint64, error) {
	panic("not supported")
}

func (p *DB) TruncateTail(n uint64) (uint64, error) {
	panic("not supported")
}

func (p *DB) Sync() error {
	panic("not supported")
}

func (p *DB) MigrateTable(s string, f func([]byte) ([]byte, error)) error {
	panic("not supported")
}

func (p *DB) AncientDatadir() (string, error) {
	panic("not supported")
}

var _ ethdb.Database = (*DB)(nil)
