package main

import (
	"fmt"

	"utils/testutil"
)

func main() {
	testutil.RunTest(TestSemaphore, "TestSemaphore")
	testutil.RunTest(TestMutex, "TestMutex")
	testutil.RunTest(TestMutexFairness, "TestMutexFairness")

	fmt.Println("Mutex test passed")
}
