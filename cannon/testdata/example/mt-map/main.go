package main

import (
	"fmt"

	"utils/testutil"
)

func main() {
	testutil.RunTest(TestMapMatchesRWMutex, "TestMapMatchesRWMutex")
	testutil.RunTest(TestMapMatchesDeepCopy, "TestMapMatchesDeepCopy")
	testutil.RunTest(TestConcurrentRange, "TestConcurrentRange")
	testutil.RunTest(TestIssue40999, "TestIssue40999")
	testutil.RunTest(TestMapRangeNestedCall, "TestMapRangeNestedCall")
	testutil.RunTest(TestCompareAndSwap_NonExistingKey, "TestCompareAndSwap_NonExistingKey")
	testutil.RunTest(TestMapRangeNoAllocations, "TestMapRangeNoAllocations")

	fmt.Println("Map test passed")
}
