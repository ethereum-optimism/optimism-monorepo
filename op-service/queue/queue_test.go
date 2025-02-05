package queue

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestQueue(t *testing.T) {
	t.Run("enqueue amd dequeue", func(t *testing.T) {
		q := Queue[int]{}
		q.Enqueue(1, 2, 3, 4)

		p, peekOk := q.Peek()
		require.True(t, peekOk)
		require.Equal(t, 1, p)

		d, dequeueOk := q.Dequeue()
		require.Equal(t, 1, d)
		require.True(t, dequeueOk)
		require.Equal(t, 3, q.Len())
		p, peekOk = q.Peek()
		require.True(t, peekOk)
		require.Equal(t, 2, p)

		d, dequeueOk = q.Dequeue()
		require.Equal(t, 2, d)
		require.True(t, dequeueOk)
		require.Equal(t, 2, q.Len())
		p, peekOk = q.Peek()
		require.True(t, peekOk)
		require.Equal(t, 3, p)

		d, dequeueOk = q.Dequeue()
		require.Equal(t, 3, d)
		require.True(t, dequeueOk)
		require.Equal(t, 1, q.Len())
		p, peekOk = q.Peek()
		require.True(t, peekOk)
		require.Equal(t, 4, p)

		d, dequeueOk = q.Dequeue()
		require.Equal(t, 4, d)
		require.True(t, dequeueOk)
		require.Equal(t, 0, q.Len())
		p, peekOk = q.Peek()
		require.False(t, peekOk)
		require.Equal(t, 0, p)

		d, dequeueOk = q.Dequeue()
		require.Equal(t, 0, d)
		require.False(t, dequeueOk)
		require.Equal(t, 0, q.Len())
		p, peekOk = q.Peek()
		require.False(t, peekOk)
		require.Equal(t, 0, p)
		p, peekOk = q.Peek()
		require.False(t, peekOk)
		require.Equal(t, 0, p)
	})

	t.Run("peekN and deqeueueN", func(t *testing.T) {
		q := Queue[int]{}
		q.Enqueue(1, 2, 3, 4)

		p, peekOk := q.PeekN(1)
		require.True(t, peekOk)
		require.Equal(t, 2, p)

		p, peekOk = q.PeekN(2)
		require.Equal(t, 3, p)
		require.True(t, peekOk)
		require.Equal(t, 4, q.Len())

		p, peekOk = q.PeekN(4)
		require.Equal(t, 0, p)
		require.False(t, peekOk)

		d, dequeueOk := q.DequeueN(1)
		require.Equal(t, []int{1}, d)
		require.True(t, dequeueOk)
		require.Equal(t, 3, q.Len())

		d, dequeueOk = q.DequeueN(3)
		require.Equal(t, []int{2, 3, 4}, d)
		require.True(t, dequeueOk)
		require.Equal(t, 0, q.Len())
	})

	t.Run("enqueue and clear", func(t *testing.T) {
		q := Queue[int]{}
		q.Enqueue(5, 6, 7)

		q.Clear()
		require.Equal(t, 0, q.Len())

		d, ok := q.Dequeue()
		require.Equal(t, 0, d)
		require.False(t, ok)
	})

	t.Run("prepend", func(t *testing.T) {
		var q, r Queue[int]
		q.Enqueue(5, 6, 7)
		r.Enqueue(8, 9)

		q.Prepend(r...)
		require.Equal(t, 5, q.Len())

		d, ok := q.Dequeue()
		require.Equal(t, 8, d)
		require.True(t, ok)
		require.Equal(t, 4, q.Len())

		q.Prepend()
		require.Equal(t, 4, q.Len())

		d, ok = q.Dequeue()
		require.Equal(t, 9, d)
		require.True(t, ok)
	})
}
