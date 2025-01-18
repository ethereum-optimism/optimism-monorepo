package locks

import (
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
	m.Default(10001, func() int64 {
		t.Fatal("should not replace existing value")
		return 0
	})
	m.Default(10002, func() int64 {
		return 42
	})
	v, ok = m.Get(10002)
	require.True(t, ok)
	require.Equal(t, int64(42), v)
}

func TestRWMap_DefaultOnEmpty(t *testing.T) {
	m := &RWMap[uint64, int64]{}
	// this should work, even if the first call to the map.
	m.Default(10002, func() int64 {
		return 42
	})
	v, ok := m.Get(10002)
	require.True(t, ok)
	require.Equal(t, int64(42), v)
}
