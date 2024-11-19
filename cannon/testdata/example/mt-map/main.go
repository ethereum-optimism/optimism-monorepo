package main

import (
	"fmt"
	"sync"
)

func main() {
	var m sync.Map

	m.Store("hello", "world")
	m.Store("foo", "bar")
	m.Store("baz", "qux")

	m.Delete("foo")
	m.Load("baz")

	go func() {
		m.CompareAndDelete("hello", "world")
		m.LoadAndDelete("baz")
	}()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			m.Load("hello")
			m.Load("baz")
			m.Range(func(k, v interface{}) bool {
				m.Load("hello")
				m.Load("baz")
				return true
			})
			m.CompareAndSwap("hello", "world", "Go")
			m.LoadOrStore("hello", "world")
			wg.Done()
		}()
	}

	wg.Wait()

	fmt.Println("Map test passed")
}
