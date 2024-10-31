package locks

import "sync"

// RWValue is a simple container struct, to deconflict reads/writes of the value,
// without locking up a bigger structure in the caller.
// It exposes the underlying RWLock and Value for direct access where needed.
type RWValue[E any] struct {
	sync.RWMutex
	Value E
}

func (c *RWValue[E]) Get() (out E) {
	c.RLock()
	defer c.RUnlock()
	out = c.Value
	return
}

func (c *RWValue[E]) Set(v E) {
	c.Lock()
	defer c.Unlock()
	c.Value = v
}
