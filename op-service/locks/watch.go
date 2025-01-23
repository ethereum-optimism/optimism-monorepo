package locks

import (
	"context"
	"sync"
)

// Watch makes a value watch-able: every change will be notified to those watching.
type Watch[E any] struct {
	mu       sync.RWMutex
	value    E
	watchers map[chan E]struct{}
}

func (c *Watch[E]) Get() (out E) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out = c.value
	return
}

// Set changes the value. This blocks until all watching subscribers have accepted the value.
func (c *Watch[E]) Set(v E) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.value = v
	for ch := range c.watchers {
		ch <- v
	}
}

// Watch adds a subscriber. Make sure it has channel buffer capacity, since subscribers block.
func (c *Watch[E]) Watch(dest chan E) (cancel func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.watchers == nil {
		c.watchers = make(map[chan E]struct{})
	}
	c.watchers[dest] = struct{}{}
	return func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		delete(c.watchers, dest)
	}
}

func (c *Watch[E]) Catch(ctx context.Context, condition func(E) bool) (E, error) {
	if x := c.Get(); condition(x) { // happy-path, no need to start a watcher
		return x, nil
	}

	out := make(chan E, 10)
	cancelWatch := c.Watch(out)
	defer cancelWatch()

	for {
		select {
		case <-ctx.Done():
			var x E
			return x, ctx.Err()
		case x := <-out:
			if condition(x) {
				return x, nil
			}
		}
	}
}
