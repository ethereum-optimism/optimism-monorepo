package helpers

import (
	"context"

	op_e2e "github.com/ethereum-optimism/optimism/op-e2e"

	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils"
)

// Testing is an interface to Go-like testing,
// extended with a context getter for the test runner to shut down individual actions without interrupting the test,
// and a signaling function for when an invalid action is hit.
// This helps custom test runners navigate slow or invalid actions, e.g. during fuzzing.
type Testing interface {
	e2eutils.TestingBase
	// Ctx shares a context to execute an action with, the test runner may interrupt the action without stopping the test.
	Ctx() context.Context
	// InvalidAction indicates the failure is due to action incompatibility, does not stop the test.
	InvalidAction(format string, args ...any)
}

// Action is a function that may change the state of one or more actors or check their state.
// Action definitions are meant to be very small building blocks,
// and then composed into larger patterns to write more elaborate tests.
type Action func(t Testing)

// ActionStatus defines the state of an action, to make a basic distinction between InvalidAction() and other calls.
type ActionStatus uint

const (
	// ActionOK indicates the action is valid to apply
	ActionOK ActionStatus = iota
	// ActionInvalid indicates the action is not applicable, and a different next action may taken.
	ActionInvalid
	// More action status types may be used to indicate e.g. required rewinds,
	// simple skips, or special cases for fuzzing.
)

// defaultTesting is a simple implementation of Testing that takes standard Go testing framework,
// and handles invalid actions as errors, and exposes a Reset function to change the context and action state,
// to recover after an invalid action or cancelled context.
type defaultTesting struct {
	e2eutils.TestingBase
	ctx   context.Context
	state ActionStatus
}

type StatefulTesting interface {
	Testing
	Reset(actionCtx context.Context)
	State() ActionStatus
}

// NewDefaultTesting returns a new testing obj, and enables parallel test execution.
// Returns an interface, we're likely changing the behavior here as we build more action tests.
func NewDefaultTesting(tb e2eutils.TestingBase) StatefulTesting {
	op_e2e.InitParallel(tb)

	return &defaultTesting{
		TestingBase: tb,
		ctx:         context.Background(),
		state:       ActionOK,
	}
}

func SubTest(tb e2eutils.TestingBase) StatefulTesting {
	return &defaultTesting{
		TestingBase: tb,
		ctx:         context.Background(),
		state:       ActionOK,
	}
}

// Ctx shares a context to execute an action with, the test runner may interrupt the action without stopping the test.
func (st *defaultTesting) Ctx() context.Context {
	return st.ctx
}

// InvalidAction indicates the failure is due to action incompatibility, does not stop the test.
// The format and args behave the same as fmt.Sprintf, testing.T.Errorf, etc.
func (st *defaultTesting) InvalidAction(format string, args ...any) {
	st.TestingBase.Helper() // report the error on the call-site to make debugging clear, not here.
	st.Errorf("invalid action err: "+format, args...)
	st.state = ActionInvalid
}

// Reset prepares the testing util for the next action, changing the context and state back to OK.
func (st *defaultTesting) Reset(actionCtx context.Context) {
	st.state = ActionOK
	st.ctx = actionCtx
}

// State shares the current action state.
func (st *defaultTesting) State() ActionStatus {
	return st.state
}

var _ Testing = (*defaultTesting)(nil)
