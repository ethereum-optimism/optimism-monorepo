package locks

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRWMap(t *testing.T) {
	m := &RWMap[uint64, int64]{}

	// get on new map
	v, ok := m.Get(123)
	require.False(t, ok)
	require.Equal(t, int64(0), v)

	// set a value
	m.Set(123, 42)
	v, ok = m.Get(123)
	require.True(t, ok)
	require.Equal(t, int64(42), v)

	// overwrite a value
	m.Set(123, -42)
	v, ok = m.Get(123)
	require.True(t, ok)
	require.Equal(t, int64(-42), v)

	// add a value
	m.Set(10, 100)

	// range over values
	got := make(map[uint64]int64)
	m.Range(func(key uint64, value int64) bool {
		if _, ok := got[key]; ok {
			panic("duplicate")
		}
		got[key] = value
		return true
	})
	require.Len(t, got, 2)
	require.Equal(t, int64(100), got[uint64(10)])
	require.Equal(t, int64(-42), got[uint64(123)])

	// range and stop early
	clear(got)
	m.Range(func(key uint64, value int64) bool {
		got[key] = value
		return false
	})
	require.Len(t, got, 1, "stop early")

	// remove a value
	require.True(t, m.Has(10))
	m.Delete(10)
	require.False(t, m.Has(10))
	// and add it back, sanity check
	m.Set(10, 123)
	require.True(t, m.Has(10))

	// remove a non-existent value
	m.Delete(132983213)

	m.Set(10001, 100)
	m.CreateIfMissing(10001, func() int64 {
		t.Fatal("should not replace existing value")
		return 0
	})
	m.CreateIfMissing(10002, func() int64 {
		return 42
	})
	v, ok = m.Get(10002)
	require.True(t, ok)
	require.Equal(t, int64(42), v)

	require.True(t, m.SetIfMissing(10003, 111))
	require.False(t, m.SetIfMissing(10003, 123))
	v, ok = m.Get(10003)
	require.True(t, ok)
	require.Equal(t, int64(111), v)
}

func TestRWMap_DefaultOnEmpty(t *testing.T) {
	m := &RWMap[uint64, int64]{}
	// this should work, even if the first call to the map.
	m.CreateIfMissing(10002, func() int64 {
		return 42
	})
	v, ok := m.Get(10002)
	require.True(t, ok)
	require.Equal(t, int64(42), v)
}

func TestRWMap_KeysValues(t *testing.T) {
	m := &RWMap[uint64, int64]{}

	require.Empty(t, m.Keys())
	require.Empty(t, m.Values())

	m.Set(1, 100)
	m.Set(2, 200)
	m.Set(3, 300)

	length := m.Len()

	keys := m.Keys()
	require.Equal(t, length, len(keys))
	slices.Sort(keys)
	require.Equal(t, []uint64{1, 2, 3}, keys)

	values := m.Values()
	require.Equal(t, length, len(values))
	slices.Sort(values)
	require.Equal(t, []int64{100, 200, 300}, values)

	m.Clear()

	require.Empty(t, m.Keys())
	require.Empty(t, m.Values())
}

func TestRWMapFromMap(t *testing.T) {
	m := RWMapFromMap(map[uint64]int64{
		1: 10,
		2: 20,
		3: 30,
	})
	require.Equal(t, 3, m.Len())
}
