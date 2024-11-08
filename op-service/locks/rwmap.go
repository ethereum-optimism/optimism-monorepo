package locks

import "sync"

// RWMap is a simple wrapper around a map, with global Read-Write protection.
// For many concurrent reads/writes a sync.Map may be more performant,
// although it does not utilize Go generics.
// The RWMap does not have to be initialized,
// it is immediately ready for reads/writes.
type RWMap[K comparable, V any] struct {
	inner map[K]V
	mu    sync.RWMutex
}

func (m *RWMap[K, V]) Has(key K) (ok bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok = m.inner[key]
	return
}

func (m *RWMap[K, V]) Get(key K) (value V, ok bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	value, ok = m.inner[key]
	return
}

func (m *RWMap[K, V]) Set(key K, value V) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.inner == nil {
		m.inner = make(map[K]V)
	}
	m.inner[key] = value
}

// Range calls f sequentially for each key and value present in the map.
// If f returns false, range stops the iteration.
func (m *RWMap[K, V]) Range(f func(key K, value V) bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for k, v := range m.inner {
		if !f(k, v) {
			break
		}
	}
}

// Clear removes all key-value pairs from the map.
func (m *RWMap[K, V]) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	clear(m.inner)
}
