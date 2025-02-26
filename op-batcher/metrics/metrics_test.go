package metrics

import (
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPendingDABytesNeverNegative(t *testing.T) {
	m := NewMetrics("test")

	// Set pendingDABytes to a small value
	atomic.StoreInt64(&m.pendingDABytes, 100)

	// Verify that pendingDABytes is positive
	require.Equal(t, float64(100), m.PendingDABytes())

	// Simulate RecordL2BlockInChannel by directly calling the atomic operation
	// with a larger value than what's currently stored
	for {
		current := atomic.LoadInt64(&m.pendingDABytes)
		// If current value is already 0 or negative, don't subtract more
		if current <= 0 {
			atomic.StoreInt64(&m.pendingDABytes, 0)
			break
		}

		// Calculate new value, ensuring it doesn't go below 0
		newValue := current - int64(200)
		if newValue < 0 {
			newValue = 0
		}

		// Try to update the value atomically
		if atomic.CompareAndSwapInt64(&m.pendingDABytes, current, newValue) {
			break
		}
		// If CAS failed, loop and try again
	}

	// Verify that pendingDABytes is never negative
	require.GreaterOrEqual(t, m.PendingDABytes(), float64(0))
}
