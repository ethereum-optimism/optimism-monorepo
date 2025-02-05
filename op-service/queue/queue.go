package queue

// Queue implements a FIFO queue.
type Queue[T any] []T

// Enqueue adds the elements to the back of the queue.
func (q *Queue[T]) Enqueue(t ...T) {
	if len(t) == 0 {
		return
	}
	*q = append(*q, t...)
}

// Dequeue removes a single element from the front of the queue
// (if there is one) and returns it. Returns a zero value and false
// if there is no element to dequeue.
func (q *Queue[T]) Dequeue() (T, bool) {
	if len(*q) == 0 {
		var zeroValue T
		return zeroValue, false
	}
	t := (*q)[0]
	*q = (*q)[1:]
	return t, true
}

// DequeueN removes N elements from the front of the queue
// (if there are enough) and returns a slice of those elements. Returns
// a nil slice and false if there are insufficient elements to dequeue.
func (q *Queue[T]) DequeueN(N int) ([]T, bool) {
	if len(*q) < N {
		return nil, false
	}
	t := (*q)[0:N]
	*q = (*q)[N:]
	return t, true
}

// Prepend inserts the elements at the front of the queue,
// preserving their order. A noop if t is empty.
func (q *Queue[T]) Prepend(t ...T) {
	if len(t) == 0 {
		return
	}
	*q = append(t, *q...)
}

// Clear removes all elements from the queue.
func (q *Queue[T]) Clear() {
	*q = (*q)[:0]
}

// Len returns the number of elements in the queue.
func (q *Queue[T]) Len() int {
	return len(*q)
}

// Peek returns the single element at the front of the queue
// (if there is one) without removing it. Returns a zero value and
// false if there is no element to peek at.
func (q *Queue[T]) Peek() (T, bool) {
	return q.PeekN(0)
}

// PeekN returns the element in Nth position in the queue
// Returns a zero value and false if there are insufficient elements
// in the queue.
func (q *Queue[T]) PeekN(N int) (T, bool) {
	if len(*q) <= N {
		var zeroValue T
		return zeroValue, false
	}
	t := (*q)[N]
	return t, true
}
