package main

import (
	"fmt"
	"os"
	"testing"
)

func main() {
	runTest(TestSwapInt32, "TestSwapInt32")
	runTest(TestSwapInt32Method, "TestSwapInt32Method")
	runTest(TestSwapUint32, "TestSwapUint32")
	runTest(TestSwapUint32Method, "TestSwapUint32Method")
	runTest(TestSwapInt64, "TestSwapInt64")
	runTest(TestSwapInt64Method, "TestSwapInt64Method")
	runTest(TestSwapUint64, "TestSwapUint64")
	runTest(TestSwapUint64Method, "TestSwapUint64Method")
	runTest(TestSwapUintptr, "TestSwapUintptr")
	runTest(TestSwapUintptrMethod, "TestSwapUintptrMethod")
	runTest(TestSwapPointer, "TestSwapPointer")
	runTest(TestSwapPointerMethod, "TestSwapPointerMethod")
	runTest(TestAddInt32, "TestAddInt32")
	runTest(TestAddInt32Method, "TestAddInt32Method")
	runTest(TestAddUint32, "TestAddUint32")
	runTest(TestAddUint32Method, "TestAddUint32Method")
	runTest(TestAddInt64, "TestAddInt64")
	runTest(TestAddInt64Method, "TestAddInt64Method")
	runTest(TestAddUint64, "TestAddUint64")
	runTest(TestAddUint64Method, "TestAddUint64Method")
	runTest(TestAddUintptr, "TestAddUintptr")
	runTest(TestAddUintptrMethod, "TestAddUintptrMethod")
	runTest(TestCompareAndSwapInt32, "TestCompareAndSwapInt32")
	runTest(TestCompareAndSwapInt32Method, "TestCompareAndSwapInt32Method")
	runTest(TestCompareAndSwapUint32, "TestCompareAndSwapUint32")
	runTest(TestCompareAndSwapUint32Method, "TestCompareAndSwapUint32Method")
	runTest(TestCompareAndSwapInt64, "TestCompareAndSwapInt64")
	runTest(TestCompareAndSwapInt64Method, "TestCompareAndSwapInt64Method")
	runTest(TestCompareAndSwapUint64, "TestCompareAndSwapUint64")
	runTest(TestCompareAndSwapUint64Method, "TestCompareAndSwapUint64Method")
	runTest(TestCompareAndSwapUintptr, "TestCompareAndSwapUintptr")
	runTest(TestCompareAndSwapUintptrMethod, "TestCompareAndSwapUintptrMethod")
	runTest(TestCompareAndSwapPointer, "TestCompareAndSwapPointer")
	runTest(TestCompareAndSwapPointerMethod, "TestCompareAndSwapPointerMethod")
	runTest(TestLoadInt32, "TestLoadInt32")
	runTest(TestLoadInt32Method, "TestLoadInt32Method")
	runTest(TestLoadUint32, "TestLoadUint32")
	runTest(TestLoadUint32Method, "TestLoadUint32Method")
	runTest(TestLoadInt64, "TestLoadInt64")
	runTest(TestLoadInt64Method, "TestLoadInt64Method")
	runTest(TestLoadUint64, "TestLoadUint64")
	runTest(TestLoadUint64Method, "TestLoadUint64Method")
	runTest(TestLoadUintptr, "TestLoadUintptr")
	runTest(TestLoadUintptrMethod, "TestLoadUintptrMethod")
	runTest(TestLoadPointer, "TestLoadPointer")
	runTest(TestLoadPointerMethod, "TestLoadPointerMethod")
	runTest(TestStoreInt32, "TestStoreInt32")
	runTest(TestStoreInt32Method, "TestStoreInt32Method")
	runTest(TestStoreUint32, "TestStoreUint32")
	runTest(TestStoreUint32Method, "TestStoreUint32Method")
	runTest(TestStoreInt64, "TestStoreInt64")
	runTest(TestStoreInt64Method, "TestStoreInt64Method")
	runTest(TestStoreUint64, "TestStoreUint64")
	runTest(TestStoreUint64Method, "TestStoreUint64Method")
	runTest(TestStoreUintptr, "TestStoreUintptr")
	runTest(TestStoreUintptrMethod, "TestStoreUintptrMethod")
	runTest(TestStorePointer, "TestStorePointer")
	runTest(TestStorePointerMethod, "TestStorePointerMethod")
	runTest(TestHammer32, "TestHammer32")
	runTest(TestHammer64, "TestHammer64")
	runTest(TestAutoAligned64, "TestAutoAligned64")
	runTest(TestNilDeref, "TestNilDeref")

	// TODO(#12162) These tests are currently failing - need to debug
	//runTest(TestHammerStoreLoad, "TestHammerStoreLoad")
	//runTest(TestStoreLoadSeqCst32, "TestStoreLoadSeqCst32")
	//runTest(TestStoreLoadSeqCst64, "TestStoreLoadSeqCst64")
	//runTest(TestStoreLoadRelAcq32, "TestStoreLoadRelAcq32")
	//runTest(TestStoreLoadRelAcq64, "TestStoreLoadRelAcq64")
	// This one fails natively:
	//runTest(TestUnaligned64, "TestUnaligned64")

	fmt.Println("Atomic tests passed")
}

func runTest(testFunc func(*testing.T), name string) {
	t := &testing.T{}
	testFunc(t)
	if t.Failed() {
		fmt.Printf("Test failed: %v\n", name)
		os.Exit(1)
	} else {
		fmt.Printf("Test passed: %v\n", name)
	}
}
