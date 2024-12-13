package main

import (
	"fmt"

	"utils/testutil"
)

func main() {
	testutil.RunTest(TestPool, "TestPool")
	testutil.RunTest(TestPoolNew, "TestPoolNew")
	testutil.RunTest(TestPoolGC, "TestPoolGC")
	testutil.RunTest(TestPoolRelease, "TestPoolRelease")
	testutil.RunTest(TestPoolStress, "TestPoolStress")
	testutil.RunTest(TestPoolDequeue, "TestPoolDequeue")
	testutil.RunTest(TestPoolChain, "TestPoolChain")
	testutil.RunTest(TestNilPool, "TestNilPool")

	fmt.Println("Pool test passed")
}
