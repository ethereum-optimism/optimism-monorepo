package systest

import (
	"context"
	"time"
)

type BasicT = testingTB

type testingTB interface {
	Cleanup(func())
	Error(args ...any)
	Errorf(format string, args ...any)
	Fail()
	Failed() bool
	FailNow()
	Fatal(args ...any)
	Fatalf(format string, args ...any)
	Helper()
	Log(args ...any)
	Logf(format string, args ...any)
	Name() string
	Setenv(key, value string)
	Skip(args ...any)
	SkipNow()
	Skipf(format string, args ...any)
	Skipped() bool
	TempDir() string
}

type tContext interface {
	Context() context.Context
}

type T interface {
	testingTB
	Context() context.Context
	WithContext(ctx context.Context) T
	Deadline() (deadline time.Time, ok bool)
	Parallel()
	Run(string, func(t T))
}

func NewT(t testingTB) T {
	t.Helper()
	if tt, ok := t.(T); ok {
		return tt
	}
	ctx := context.TODO()
	if tt, ok := t.(tContext); ok {
		ctx = tt.Context()
	}
	return &tbWrapper{
		testingTB: t,
		ctx:       ctx,
	}
}
