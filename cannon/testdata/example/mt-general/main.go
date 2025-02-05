package main

import (
	"fmt"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
)

func main() {
	// try some concurrency!
	var wg sync.WaitGroup
	wg.Add(2)
	var x atomic.Int32
	go func() {
		x.Add(2)
		wg.Done()
	}()
	go func() {
		x.Add(40)
		wg.Done()
	}()
	wg.Wait()
	fmt.Printf("waitgroup result: %d\n", x.Load())

	// channels
	a := make(chan int, 1)
	b := make(chan int)
	c := make(chan int)
	go func() {
		t0 := <-a
		b <- t0
	}()
	go func() {
		t1 := <-b
		c <- t1
	}()
	a <- 1234
	out := <-c
	fmt.Printf("channels result: %d\n", out)

	// try a GC! (the runtime might not have run one yet)
	runtime.GC()
	_, _ = os.Stdout.Write([]byte("GC complete!\n"))
}
