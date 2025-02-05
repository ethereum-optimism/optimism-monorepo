package main

import (
	_ "unsafe" // Required for go:linkname
)

var Runtime_Semacquire = runtime_Semacquire
var Runtime_Semrelease = runtime_Semrelease

//go:linkname runtime_Semacquire sync.runtime_Semacquire
func runtime_Semacquire(s *uint32)

//go:linkname runtime_Semrelease sync.runtime_Semrelease
func runtime_Semrelease(s *uint32, handoff bool, skipframes int)
