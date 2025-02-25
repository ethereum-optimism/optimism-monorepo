package locks

import (
	"maps"
	"sync"
)

// RWMap is a simple wrapper around a map, with global Read-Write protection.
// For many concurrent reads/writes a sync.Map may be more performant,
// although it does not utilize Go generics.
// The RWMap does not have to be initialized,
// it is immediately ready for reads/writes.
type RWMap[K comparable, V any] struct {
	inner map[K]V
	mu    sync.RWMutex
}

// RWMapFromMap creates a RWMap from the given map.
// This shallow-copies the map, changes to the original map will not affect the new RWMap.
func RWMapFromMap[K comparable, V any](m map[K]V) *RWMap[K, V] {
	return &RWMap[K, V]{inner: maps.Clone(m)}
}

// CreateIfMissing creates a value at the given key, if the key is not set yet.
func (m *RWMap[K, V]) CreateIfMissing(key K, fn func() V) (changed bool) {
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

// SetIfMissing is a convenience function to set a missing value if it does not already exist.
// To lazy-init the value, see CreateIfMissing.
func (m *RWMap[K, V]) SetIfMissing(key K, v V) (changed bool) {
	return m.CreateIfMissing(key, func() V {
		return v
	})
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

// Keys returns an unsorted list of keys of the map.
func (m *RWMap[K, V]) Keys() (out []K) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out = make([]K, 0, len(m.inner))
	for k := range m.inner {
		out = append(out, k)
	}
	return out
}

// Values returns an unsorted list of values of the map.
func (m *RWMap[K, V]) Values() (out []V) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out = make([]V, 0, len(m.inner))
	for _, v := range m.inner {
		out = append(out, v)
	}
	return out
}

// Clear removes all key-value pairs from the map.
func (m *RWMap[K, V]) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	clear(m.inner)
}
