package locks

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRWValue(t *testing.T) {
	v := &RWValue[uint64]{}
	require.Equal(t, uint64(0), v.Get())
	v.Set(123)
	require.Equal(t, uint64(123), v.Get())
	v.Set(42)
	require.Equal(t, uint64(42), v.Get())
}
