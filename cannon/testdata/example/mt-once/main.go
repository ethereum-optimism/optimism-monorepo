// Portions of this code are derived from code written by The Go Authors.
// See original source: https://github.com/golang/go/blob/400433af3660905ecaceaf19ddad3e6c24b141df/src/sync/once_test.go
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
	"sync"
)

func main() {
	TestOnce()
	TestOncePanic()

	fmt.Println("Once test passed")
}

type one int

func (o *one) Increment() {
	*o++
}

func run(once *sync.Once, o *one, c chan bool) {
	once.Do(func() { o.Increment() })
	if v := *o; v != 1 {
		_, _ = fmt.Fprintf(os.Stderr, "once failed inside run: %d is not 1\n", v)
		os.Exit(1)
	}
	c <- true
}

func TestOnce() {
	o := new(one)
	once := new(sync.Once)
	c := make(chan bool)
	const N = 10
	for i := 0; i < N; i++ {
		go run(once, o, c)
	}
	for i := 0; i < N; i++ {
		<-c
	}
	if *o != 1 {
		_, _ = fmt.Fprintf(os.Stderr, "once failed outside run: %d is not 1\n", *o)
		os.Exit(1)
	}
}

func TestOncePanic() {
	var once sync.Once
	func() {
		defer func() {
			if r := recover(); r == nil {
				_, _ = fmt.Fprintf(os.Stderr, "Once.Do did not panic")
				os.Exit(1)
			}
		}()
		once.Do(func() {
			panic("failed")
		})
	}()

	once.Do(func() {
		_, _ = fmt.Fprintf(os.Stderr, "Once.Do called twice")
		os.Exit(1)
	})
}
