package main

import (
	"fmt"
	"testing"

	"utils/testutil"
)

func main() {
	testutil.RunTest(ShouldFail, "ShouldFail")

	fmt.Println("Passed test that should have failed")
}

func ShouldFail(t *testutil.TestRunner) {
	t.Run("panic test", func(t testing.TB) {
		panic("oops")
	})
}
