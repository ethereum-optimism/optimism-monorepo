package main

import (
	"fmt"

	"utils/testutil"
)

func main() {
	testutil.RunTest(TestOnceFunc, "TestOnceFunc")
	testutil.RunTest(TestOnceValue, "TestOnceValue")
	testutil.RunTest(TestOnceValues, "TestOnceValues")
	testutil.RunTest(TestOnceFuncPanic, "TestOnceFuncPanic")
	testutil.RunTest(TestOnceValuePanic, "TestOnceValuePanic")
	testutil.RunTest(TestOnceValuesPanic, "TestOnceValuesPanic")
	testutil.RunTest(TestOnceFuncPanicNil, "TestOnceFuncPanicNil")
	testutil.RunTest(TestOnceFuncGoexit, "TestOnceFuncGoexit")
	testutil.RunTest(TestOnceFuncPanicTraceback, "TestOnceFuncPanicTraceback")
	testutil.RunTest(TestOnceXGC, "TestOnceXGC")

	fmt.Println("OnceFunc tests passed")
}
