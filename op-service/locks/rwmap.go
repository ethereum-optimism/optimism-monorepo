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

// Default creates a value at the given key, if the key is not set yet.
func (m *RWMap[K, V]) Default(key K, fn func() V) (changed bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.inner == nil {
		m.inner = make(map[K]V)
	}
	_, ok := m.inner[key]
	if !ok {
		m.inner[key] = fn()
	}
	return !ok // if it exists, nothing changed
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

func (m *RWMap[K, V]) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.inner)
}

func (m *RWMap[K, V]) Delete(key K) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.inner, key)
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

// InitPtrMaybe sets a pointer-value in the map, if it's not set yet, to a new object.
func InitPtrMaybe[K comparable, V any](m *RWMap[K, *V], key K) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.inner == nil {
		m.inner = make(map[K]*V)
	}
	_, ok := m.inner[key]
	if !ok {
		m.inner[key] = new(V)
	}
}
