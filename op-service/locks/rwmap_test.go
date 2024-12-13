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
}
