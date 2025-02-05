// This file is based on code written by The Go Authors.
// See original source: https://github.com/golang/go/blob/go1.22.7/src/sync/oncefunc_test.go
//
// --- Original License Notice ---
//
// Copyright (c) 2009 The Go Authors. All rights reserved.
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
// * Neither the name of Google Inc. nor the names of its
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
	"bytes"
	"math"
	"runtime"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"testing"
	_ "unsafe"

	"utils/testutil"
)

// We assume that the Once.Do tests have already covered parallelism.

func TestOnceFunc(t *testutil.TestRunner) {
	calls := 0
	f := sync.OnceFunc(func() { calls++ })
	allocs := testing.AllocsPerRun(10, f)
	if calls != 1 {
		t.Errorf("want calls==1, got %d", calls)
	}
	if allocs != 0 {
		t.Errorf("want 0 allocations per call, got %v", allocs)
	}
}

func TestOnceValue(t *testutil.TestRunner) {
	calls := 0
	f := sync.OnceValue(func() int {
		calls++
		return calls
	})
	allocs := testing.AllocsPerRun(10, func() { f() })
	value := f()
	if calls != 1 {
		t.Errorf("want calls==1, got %d", calls)
	}
	if value != 1 {
		t.Errorf("want value==1, got %d", value)
	}
	if allocs != 0 {
		t.Errorf("want 0 allocations per call, got %v", allocs)
	}
}

func TestOnceValues(t *testutil.TestRunner) {
	calls := 0
	f := sync.OnceValues(func() (int, int) {
		calls++
		return calls, calls + 1
	})
	allocs := testing.AllocsPerRun(10, func() { f() })
	v1, v2 := f()
	if calls != 1 {
		t.Errorf("want calls==1, got %d", calls)
	}
	if v1 != 1 || v2 != 2 {
		t.Errorf("want v1==1 and v2==2, got %d and %d", v1, v2)
	}
	if allocs != 0 {
		t.Errorf("want 0 allocations per call, got %v", allocs)
	}
}

func testOncePanicX(t testing.TB, calls *int, f func()) {
	testOncePanicWith(t, calls, f, func(label string, p any) {
		if p != "x" {
			t.Fatalf("%s: want panic %v, got %v", label, "x", p)
		}
	})
}

func testOncePanicWith(t testing.TB, calls *int, f func(), check func(label string, p any)) {
	// Check that the each call to f panics with the same value, but the
	// underlying function is only called once.
	for _, label := range []string{"first time", "second time"} {
		var p any
		panicked := true
		func() {
			defer func() {
				p = recover()
			}()
			f()
			panicked = false
		}()
		if !panicked {
			t.Fatalf("%s: f did not panic", label)
		}
		check(label, p)
	}
	if *calls != 1 {
		t.Errorf("want calls==1, got %d", *calls)
	}
}

func TestOnceFuncPanic(t *testutil.TestRunner) {
	calls := 0
	f := sync.OnceFunc(func() {
		calls++
		panic("x")
	})
	testOncePanicX(t, &calls, f)
}

func TestOnceValuePanic(t *testutil.TestRunner) {
	calls := 0
	f := sync.OnceValue(func() int {
		calls++
		panic("x")
	})
	testOncePanicX(t, &calls, func() { f() })
}

func TestOnceValuesPanic(t *testutil.TestRunner) {
	calls := 0
	f := sync.OnceValues(func() (int, int) {
		calls++
		panic("x")
	})
	testOncePanicX(t, &calls, func() { f() })
}

func TestOnceFuncPanicNil(t *testutil.TestRunner) {
	calls := 0
	f := sync.OnceFunc(func() {
		calls++
		panic(nil)
	})
	testOncePanicWith(t, &calls, f, func(label string, p any) {
		switch p.(type) {
		case nil, *runtime.PanicNilError:
			return
		}
		t.Fatalf("%s: want nil panic, got %v", label, p)
	})
}

func TestOnceFuncGoexit(t *testutil.TestRunner) {
	// If f calls Goexit, the results are unspecified. But check that f doesn't
	// get called twice.
	calls := 0
	f := sync.OnceFunc(func() {
		calls++
		runtime.Goexit()
	})
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { recover() }()
			f()
		}()
		wg.Wait()
	}
	if calls != 1 {
		t.Errorf("want calls==1, got %d", calls)
	}
}

func TestOnceFuncPanicTraceback(t *testutil.TestRunner) {
	// Test that on the first invocation of a OnceFunc, the stack trace goes all
	// the way to the origin of the panic.
	f := sync.OnceFunc(onceFuncPanic)

	defer func() {
		if p := recover(); p != "x" {
			t.Fatalf("want panic %v, got %v", "x", p)
		}
		stack := debug.Stack()
		//want := "sync_test.onceFuncPanic"
		want := "main.onceFuncPanic"
		if !bytes.Contains(stack, []byte(want)) {
			t.Fatalf("want stack containing %v, got:\n%s", want, string(stack))
		}
	}()
	f()
}

func onceFuncPanic() {
	panic("x")
}

func TestOnceXGC(t *testutil.TestRunner) {
	fns := map[string]func([]byte) func(){
		"OnceFunc": func(buf []byte) func() {
			return sync.OnceFunc(func() { buf[0] = 1 })
		},
		"OnceValue": func(buf []byte) func() {
			f := sync.OnceValue(func() any { buf[0] = 1; return nil })
			return func() { f() }
		},
		"OnceValues": func(buf []byte) func() {
			f := sync.OnceValues(func() (any, any) { buf[0] = 1; return nil, nil })
			return func() { f() }
		},
	}
	for n, fn := range fns {
		t.Run(n, func(t testing.TB) {
			buf := make([]byte, 1024)
			var gc atomic.Bool
			runtime.SetFinalizer(&buf[0], func(_ *byte) {
				gc.Store(true)
			})
			f := fn(buf)
			gcwaitfin()
			if gc.Load() != false {
				t.Fatal("wrapped function garbage collected too early")
			}
			f()
			gcwaitfin()
			if gc.Load() != true {
				// Even if f is still alive, the function passed to Once(Func|Value|Values)
				// is not kept alive after the first call to f.
				t.Fatal("wrapped function should be garbage collected, but still live")
			}
			f()
		})
	}
}

// gcwaitfin performs garbage collection and waits for all finalizers to run.
func gcwaitfin() {
	runtime.GC()
	runtime_blockUntilEmptyFinalizerQueue(math.MaxInt64)
}

//go:linkname runtime_blockUntilEmptyFinalizerQueue runtime.blockUntilEmptyFinalizerQueue
func runtime_blockUntilEmptyFinalizerQueue(int64) bool
