package main

import (
	"fmt"

	"utils/testutil"
)

func main() {
	testutil.RunTest(TestWaitGroup, "TestWaitGroup")
	testutil.RunTest(TestWaitGroupMisuse, "TestWaitGroupMisuse")
	testutil.RunTest(TestWaitGroupRace, "TestWaitGroupRace")
	testutil.RunTest(TestWaitGroupAlign, "TestWaitGroupAlign")

	fmt.Println("WaitGroup tests passed")
}
