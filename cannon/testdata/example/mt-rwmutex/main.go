// Portions of this code are derived from code written by The Go Authors.
// See original source: https://github.com/golang/go/blob/400433af3660905ecaceaf19ddad3e6c24b141df/src/sync/rwmutex_test.go
//
// --- Original License Notice ---
//
// Copyright 2009 The Go Authors.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are
// met:
//
// * Redistributions of source code must retain the above copyright
// notice, this list of conditions and the following disclaimer.
// * Redistributions in binary form must reproduce the above
// copyright notice, this list of conditions and the following disclaimer
// in the documentation and/or other materials provided with the
// distribution.
// * Neither the name of Google LLC nor the names of its
// contributors may be used to endorse or promote products derived from
// this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
// "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
// LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
// A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
// OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
// SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
// LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
// DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
// THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
// (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
// OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
package main

import (
	"fmt"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
)

func main() {
	TestParallelReaders()
	TestRLocker()
	TestRWMutex()

	fmt.Println("RWMutex test passed")
}

func parallelReader(m *sync.RWMutex, clocked, cunlock, cdone chan bool) {
	m.RLock()
	clocked <- true
	<-cunlock
	m.RUnlock()
	cdone <- true
}

func doTestParallelReaders(numReaders, gomaxprocs int) {
	runtime.GOMAXPROCS(gomaxprocs)
	var m sync.RWMutex
	clocked := make(chan bool)
	cunlock := make(chan bool)
	cdone := make(chan bool)
	for i := 0; i < numReaders; i++ {
		go parallelReader(&m, clocked, cunlock, cdone)
	}
	// Wait for all parallel RLock()s to succeed.
	for i := 0; i < numReaders; i++ {
		<-clocked
	}
	for i := 0; i < numReaders; i++ {
		cunlock <- true
	}
	// Wait for the goroutines to finish.
	for i := 0; i < numReaders; i++ {
		<-cdone
	}
}

func TestParallelReaders() {
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(-1))
	doTestParallelReaders(1, 4)
	doTestParallelReaders(3, 4)
	doTestParallelReaders(4, 2)
}

func reader(rwm *sync.RWMutex, num_iterations int, activity *int32, cdone chan bool) {
	for i := 0; i < num_iterations; i++ {
		rwm.RLock()
		n := atomic.AddInt32(activity, 1)
		if n < 1 || n >= 10000 {
			rwm.RUnlock()
			panic(fmt.Sprintf("wlock(%d)\n", n))
		}
		for i := 0; i < 100; i++ {
		}
		atomic.AddInt32(activity, -1)
		rwm.RUnlock()
	}
	cdone <- true
}

func writer(rwm *sync.RWMutex, num_iterations int, activity *int32, cdone chan bool) {
	for i := 0; i < num_iterations; i++ {
		rwm.Lock()
		n := atomic.AddInt32(activity, 10000)
		if n != 10000 {
			rwm.Unlock()
			panic(fmt.Sprintf("wlock(%d)\n", n))
		}
		for i := 0; i < 100; i++ {
		}
		atomic.AddInt32(activity, -10000)
		rwm.Unlock()
	}
	cdone <- true
}

func HammerRWMutex(gomaxprocs, numReaders, num_iterations int) {
	runtime.GOMAXPROCS(gomaxprocs)
	// Number of active readers + 10000 * number of active writers.
	var activity int32
	var rwm sync.RWMutex
	cdone := make(chan bool)
	go writer(&rwm, num_iterations, &activity, cdone)
	var i int
	for i = 0; i < numReaders/2; i++ {
		go reader(&rwm, num_iterations, &activity, cdone)
	}
	go writer(&rwm, num_iterations, &activity, cdone)
	for ; i < numReaders; i++ {
		go reader(&rwm, num_iterations, &activity, cdone)
	}
	// Wait for the 2 writers and all readers to finish.
	for i := 0; i < 2+numReaders; i++ {
		<-cdone
	}
}

func TestRWMutex() {
	var m sync.RWMutex

	m.Lock()
	if m.TryLock() {
		_, _ = fmt.Fprintln(os.Stderr, "TryLock succeeded with mutex locked")
		os.Exit(1)
	}
	if m.TryRLock() {
		_, _ = fmt.Fprintln(os.Stderr, "TryRLock succeeded with mutex locked")
		os.Exit(1)
	}
	m.Unlock()

	if !m.TryLock() {
		_, _ = fmt.Fprintln(os.Stderr, "TryLock failed with mutex unlocked")
		os.Exit(1)
	}
	m.Unlock()

	if !m.TryRLock() {
		_, _ = fmt.Fprintln(os.Stderr, "TryRLock failed with mutex unlocked")
		os.Exit(1)
	}
	if !m.TryRLock() {
		_, _ = fmt.Fprintln(os.Stderr, "TryRLock failed with mutex unlocked")
		os.Exit(1)
	}
	if m.TryLock() {
		_, _ = fmt.Fprintln(os.Stderr, "TryLock succeeded with mutex rlocked")
		os.Exit(1)
	}
	m.RUnlock()
	m.RUnlock()

	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(-1))
	n := 5

	HammerRWMutex(1, 1, n)
	HammerRWMutex(1, 3, n)
	HammerRWMutex(1, 10, n)
	HammerRWMutex(4, 1, n)
	HammerRWMutex(4, 3, n)
	HammerRWMutex(4, 10, n)
	HammerRWMutex(10, 1, n)
	HammerRWMutex(10, 3, n)
	HammerRWMutex(10, 10, n)
	HammerRWMutex(10, 5, n)
}

func TestRLocker() {
	var wl sync.RWMutex
	var rl sync.Locker
	wlocked := make(chan bool, 1)
	rlocked := make(chan bool, 1)
	rl = wl.RLocker()
	n := 10
	go func() {
		for i := 0; i < n; i++ {
			rl.Lock()
			rl.Lock()
			rlocked <- true
			wl.Lock()
			wlocked <- true
		}
	}()
	for i := 0; i < n; i++ {
		<-rlocked
		rl.Unlock()
		select {
		case <-wlocked:
			_, _ = fmt.Fprintln(os.Stderr, "RLocker() didn't read-lock it")
			os.Exit(1)
		default:
		}
		rl.Unlock()
		<-wlocked
		select {
		case <-rlocked:
			_, _ = fmt.Fprintln(os.Stderr, "RLocker() didn't respect the write lock")
			os.Exit(1)
		default:
		}
		wl.Unlock()
	}
}
