package common

import (
	"testing"

	"github.com/ethereum-optimism/optimism/op-program/client/l2"
	"github.com/ethereum-optimism/optimism/op-program/host/kvstore"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/stretchr/testify/require"
)

func TestL2KeyValueStore(t *testing.T) {
	preimageKey := common.HexToHash("0xdead")
	codeKey := make([]byte, common.HashLength+len(rawdb.CodePrefix))
	copy(codeKey, rawdb.CodePrefix)
	copy(codeKey[len(rawdb.CodePrefix):], common.Hex2Bytes("0xdead"))
	value := []byte("value")
	codeValue := []byte("code")
	t.Run("Preimage", func(t *testing.T) {
		kv := kvstore.NewMemKV()
		db := NewL2KeyValueStore(kv)
		require.NoError(t, db.Put(preimageKey[:], value))

		readValue, err := db.Get(preimageKey[:])
		require.NoError(t, err)
		require.Equal(t, value, readValue)
		kvValue, err := kv.Get(preimageKey)
		require.NoError(t, err)
		require.Equal(t, value, kvValue)

		has, err := db.Has(preimageKey[:])
		require.NoError(t, err)
		require.True(t, has)
	})
	t.Run("Code", func(t *testing.T) {
		kv := kvstore.NewMemKV()
		db := NewL2KeyValueStore(kv)
		require.NoError(t, db.Put(codeKey, codeValue))

		readValue, err := db.Get(codeKey)
		require.NoError(t, err)
		require.Equal(t, codeValue, readValue)
		kvValue, err := kv.Get(common.Hash(codeKey[1:]))
		require.NoError(t, err)
		require.Equal(t, codeValue, kvValue)

		has, err := db.Has(codeKey)
		require.NoError(t, err)
		require.True(t, has)
	})
	t.Run("InvalidKey", func(t *testing.T) {
		kv := kvstore.NewMemKV()
		db := NewL2KeyValueStore(kv)
		_, err := db.Get([]byte("invalid"))
		require.ErrorIs(t, err, l2.ErrInvalidKeyLength)
		has, err := db.Has([]byte("invalid"))
		require.ErrorIs(t, err, l2.ErrInvalidKeyLength)
		require.False(t, has)
		err = db.Put([]byte("invalid"), []byte("value"))
		require.ErrorIs(t, err, l2.ErrInvalidKeyLength)
	})
	t.Run("MissingPreimage", func(t *testing.T) {
		kv := kvstore.NewMemKV()
		db := NewL2KeyValueStore(kv)
		has, err := db.Has(preimageKey[:])
		require.NoError(t, err)
		require.False(t, has)
	})
	t.Run("MissingCode", func(t *testing.T) {
		kv := kvstore.NewMemKV()
		db := NewL2KeyValueStore(kv)
		has, err := db.Has(codeKey)
		require.NoError(t, err)
		require.False(t, has)
	})
	t.Run("Batch", func(t *testing.T) {
		kv := &mockKV{data: make(map[common.Hash][]byte)}
		db := NewL2KeyValueStore(kv)
		batch := db.NewBatch()
		require.NoError(t, batch.Put(preimageKey[:], value))
		expectedBatchSize := len(preimageKey) + len(value)
		require.Equal(t, expectedBatchSize, batch.ValueSize())

		require.NoError(t, batch.Put(codeKey, codeValue))
		expectedBatchSize += len(codeKey) + len(codeValue)
		require.Equal(t, expectedBatchSize, batch.ValueSize())

		has, err := db.Has(preimageKey[:])
		require.NoError(t, err)
		require.True(t, has)
		has, err = db.Has(codeKey)
		require.NoError(t, err)
		require.True(t, has)

		require.NoError(t, batch.Write())

		has, err = db.Has(preimageKey[:])
		require.NoError(t, err)
		require.True(t, has)
		require.Equal(t, 2, kv.puts)
	})
	t.Run("Batch-Reset", func(t *testing.T) {
		kv := &mockKV{data: make(map[common.Hash][]byte)}
		db := NewL2KeyValueStore(kv)
		batch := db.NewBatch()
		require.NoError(t, batch.Put(preimageKey[:], value))
		require.NoError(t, batch.Put(codeKey, codeValue))
		batch.Reset()

		require.NoError(t, batch.Write())
		require.Zero(t, kv.puts)

		require.Equal(t, 0, batch.ValueSize())
		preimageKey2 := common.HexToHash("0xdead2")
		require.NoError(t, batch.Put(preimageKey2[:], value))
		require.NoError(t, batch.Write())
		require.Equal(t, 1, kv.puts)
	})
	t.Run("Batch-Replay", func(t *testing.T) {
		kv := &mockKV{data: make(map[common.Hash][]byte)}
		db := NewL2KeyValueStore(kv)
		batch := db.NewBatch()
		require.NoError(t, batch.Put(preimageKey[:], value))
		require.NoError(t, batch.Put(codeKey, codeValue))

		writer := &mockWriter{data: make(map[common.Hash][]byte)}
		require.NoError(t, batch.Replay(writer))
		require.Zero(t, kv.puts)
		require.Zero(t, kv.gets)
		require.Equal(t, 2, writer.puts)
		require.Equal(t, value, writer.data[preimageKey])
		// this is the raw code key, not the code preimage key
		require.Equal(t, codeValue, writer.data[common.Hash(codeKey)])
	})
}

type mockKV struct {
	puts int
	gets int
	data map[common.Hash][]byte
}

func (k *mockKV) Put(key common.Hash, value []byte) error {
	k.puts++
	k.data[key] = value
	return nil
}

func (k *mockKV) Get(key common.Hash) ([]byte, error) {
	k.gets++
	return k.data[key], nil
}

func (k *mockKV) Close() error {
	return nil
}

type mockWriter struct {
	puts    int
	deletes int
	data    map[common.Hash][]byte
}

func (w *mockWriter) Put(key []byte, value []byte) error {
	w.puts++
	w.data[common.Hash(key)] = value
	return nil
}

func (w *mockWriter) Delete(key []byte) error {
	w.deletes++
	delete(w.data, common.Hash(key))
	return nil
}
