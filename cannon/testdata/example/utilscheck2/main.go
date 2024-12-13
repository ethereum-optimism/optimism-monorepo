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
	t.Run("subtest 1", func(t testing.TB) {
		// Do something
	})

	t.Run("subtest 2", func(t testing.TB) {
		t.Fail()
	})
}
