package main

import (
	"fmt"
	"sync"
)

func main() {
	var x sync.Pool

	x.Put(1)
	x.Put(2)

	// try some concurrency!
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		x.Put(3)
		wg.Done()
	}()
	go func() {
		x.Put(4)
		wg.Done()
	}()

	wg.Wait()

	wg.Add(4)
	for i := 0; i < 4; i++ {
		go func() {
			x.Get()
			wg.Done()
		}()
	}
	wg.Wait()

	fmt.Println("Pool test passed")
}
