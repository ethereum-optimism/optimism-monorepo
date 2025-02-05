package testutil

import (
	"fmt"
	"os"
	"runtime"
	"sync"
	"testing"
)

func RunTest(testFunc func(*TestRunner), name string) {
	goRunTest(name, testFunc, newTestRunner(name))
}

type TestRunner struct {
	*mockT
	baseName string
}

func newTestRunner(baseName string) *TestRunner {
	return &TestRunner{mockT: newMockT(), baseName: baseName}
}

func (r *TestRunner) Run(name string, testFunc func(t testing.TB)) bool {
	testName := r.baseName
	if name != "" {
		testName = fmt.Sprintf("%v (%v)", r.baseName, name)
	}

	var tester testing.TB = r
	goRunTest(testName, testFunc, tester)
	return !r.Failed()
}

func goRunTest[T testing.TB](testName string, testFunc func(t T), t T) {
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer func() {
			if err := recover(); err != nil {
				fmt.Printf("Test panicked: %v\n\t%v", testName, err)
				os.Exit(1)
			}

			if t.Failed() {
				fmt.Printf("Test failed: %v\n", testName)
				os.Exit(1)
			} else if t.Skipped() {
				fmt.Printf("Test skipped: %v\n", testName)
			} else {
				fmt.Printf("Test passed: %v\n", testName)
			}

			wg.Done()
		}()

		testFunc(t)
	}()

	wg.Wait()
}

type mockT struct {
	*testing.T
	mu      sync.Mutex
	failed  bool
	skipped bool
}

var _ testing.TB = (*mockT)(nil)

func newMockT() *mockT {
	return &mockT{}
}

func (t *mockT) Cleanup(func()) {
	t.Fatalf("Cleanup not supported")
}

func (t *mockT) Error(args ...any) {
	fmt.Print(args...)
	t.fail()
}

func (t *mockT) Errorf(format string, args ...any) {
	fmt.Printf(format, args...)
	t.fail()
}

func (t *mockT) Fail() {
	t.fail()
}

func (t *mockT) FailNow() {
	fmt.Println("Fatal")
	t.fail()
}

func (t *mockT) Failed() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.failed
}

func (t *mockT) Fatal(args ...any) {
	fmt.Print(args...)
	t.fail()
}

func (t *mockT) Fatalf(format string, args ...any) {
	fmt.Printf(format, args...)
	t.fail()
}

func (t *mockT) Helper() {}

func (t *mockT) Log(args ...any) {
	fmt.Print(args...)
}

func (t *mockT) Logf(format string, args ...any) {
	fmt.Printf(format, args...)
}

func (t *mockT) Name() string {
	return ""
}

func (t *mockT) Setenv(key, value string) {
	t.Fatalf("Setenv not supported")
}

func (t *mockT) Skip(args ...any) {
	fmt.Println(args...)
	t.skip()
}

func (t *mockT) SkipNow() {
	t.skip()
}

func (t *mockT) Skipf(format string, args ...any) {
	fmt.Printf(format, args...)
	t.skip()
}
func (t *mockT) Skipped() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.skipped
}

func (t *mockT) skip() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.skipped = true
	runtime.Goexit()
}

func (t *mockT) fail() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.failed = true
	runtime.Goexit()
}

func (t *mockT) TempDir() string {
	t.Fatalf("TempDir not supported")
	return ""
}
