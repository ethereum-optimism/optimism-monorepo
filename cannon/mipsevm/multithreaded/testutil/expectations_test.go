package testutil

import (
	"fmt"
	"testing"

	//"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/cannon/mipsevm/arch"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/multithreaded"
)

type ExpectationMutator func(e *ExpectedMTState, st *multithreaded.State)

func TestValidate_shouldCatchMutations(t *testing.T) {
	states := []*multithreaded.State{
		RandomState(0),
		RandomState(1),
		RandomState(2),
	}
	var emptyHash [32]byte
	someThread := RandomThread(123)

	cases := []struct {
		name string
		mut  ExpectationMutator
	}{
		{name: "PreimageKey", mut: func(e *ExpectedMTState, st *multithreaded.State) { e.PreimageKey = emptyHash }},
		{name: "PreimageOffset", mut: func(e *ExpectedMTState, st *multithreaded.State) { e.PreimageOffset += 1 }},
		{name: "Heap", mut: func(e *ExpectedMTState, st *multithreaded.State) { e.Heap += 1 }},
		{name: "LLReservationStatus", mut: func(e *ExpectedMTState, st *multithreaded.State) { e.LLReservationStatus = e.LLReservationStatus + 1 }},
		{name: "LLAddress", mut: func(e *ExpectedMTState, st *multithreaded.State) { e.LLAddress += 1 }},
		{name: "LLOwnerThread", mut: func(e *ExpectedMTState, st *multithreaded.State) { e.LLOwnerThread += 1 }},
		{name: "ExitCode", mut: func(e *ExpectedMTState, st *multithreaded.State) { e.ExitCode += 1 }},
		{name: "Exited", mut: func(e *ExpectedMTState, st *multithreaded.State) { e.Exited = !e.Exited }},
		{name: "Step", mut: func(e *ExpectedMTState, st *multithreaded.State) { e.Step += 1 }},
		{name: "LastHint", mut: func(e *ExpectedMTState, st *multithreaded.State) { e.LastHint = []byte{7, 8, 9, 10} }},
		{name: "MemoryRoot", mut: func(e *ExpectedMTState, st *multithreaded.State) { e.MemoryRoot = emptyHash }},
		{name: "StepsSinceLastContextSwitch", mut: func(e *ExpectedMTState, st *multithreaded.State) { e.StepsSinceLastContextSwitch += 1 }},
		{name: "Wakeup", mut: func(e *ExpectedMTState, st *multithreaded.State) { e.Wakeup += 1 }},
		{name: "TraverseRight", mut: func(e *ExpectedMTState, st *multithreaded.State) { e.TraverseRight = !e.TraverseRight }},
		{name: "NextThreadId", mut: func(e *ExpectedMTState, st *multithreaded.State) { e.NextThreadId += 1 }},
		{name: "ThreadCount", mut: func(e *ExpectedMTState, st *multithreaded.State) { e.ThreadCount += 1 }},
		{name: "RightStackSize", mut: func(e *ExpectedMTState, st *multithreaded.State) { e.RightStackSize += 1 }},
		{name: "LeftStackSize", mut: func(e *ExpectedMTState, st *multithreaded.State) { e.LeftStackSize += 1 }},
		{name: "ActiveThreadId", mut: func(e *ExpectedMTState, st *multithreaded.State) { e.ActiveThreadId += 1 }},
		{name: "Empty thread expectations", mut: func(e *ExpectedMTState, st *multithreaded.State) {
			e.threadExpectations = map[arch.Word]*ExpectedThreadState{}
		}},
		{name: "Mismatched thread expectations", mut: func(e *ExpectedMTState, st *multithreaded.State) {
			e.threadExpectations = map[arch.Word]*ExpectedThreadState{someThread.ThreadId: newExpectedThreadState(someThread)}
		}},
		{name: "Active threadId", mut: func(e *ExpectedMTState, st *multithreaded.State) {
			e.threadExpectations[st.GetCurrentThread().ThreadId].ThreadId += 1
		}},
		{name: "Active thread exitCode", mut: func(e *ExpectedMTState, st *multithreaded.State) {
			e.threadExpectations[st.GetCurrentThread().ThreadId].ExitCode += 1
		}},
		{name: "Active thread exited", mut: func(e *ExpectedMTState, st *multithreaded.State) {
			e.threadExpectations[st.GetCurrentThread().ThreadId].Exited = !st.GetCurrentThread().Exited
		}},
		{name: "Active thread futexAddr", mut: func(e *ExpectedMTState, st *multithreaded.State) {
			e.threadExpectations[st.GetCurrentThread().ThreadId].FutexAddr += 1
		}},
		{name: "Active thread futexVal", mut: func(e *ExpectedMTState, st *multithreaded.State) {
			e.threadExpectations[st.GetCurrentThread().ThreadId].FutexVal += 1
		}},
		{name: "Active thread FutexTimeoutStep", mut: func(e *ExpectedMTState, st *multithreaded.State) {
			e.threadExpectations[st.GetCurrentThread().ThreadId].FutexTimeoutStep += 1
		}},
		{name: "Active thread PC", mut: func(e *ExpectedMTState, st *multithreaded.State) {
			e.threadExpectations[st.GetCurrentThread().ThreadId].PC += 1
		}},
		{name: "Active thread NextPC", mut: func(e *ExpectedMTState, st *multithreaded.State) {
			e.threadExpectations[st.GetCurrentThread().ThreadId].NextPC += 1
		}},
		{name: "Active thread HI", mut: func(e *ExpectedMTState, st *multithreaded.State) {
			e.threadExpectations[st.GetCurrentThread().ThreadId].HI += 1
		}},
		{name: "Active thread LO", mut: func(e *ExpectedMTState, st *multithreaded.State) {
			e.threadExpectations[st.GetCurrentThread().ThreadId].LO += 1
		}},
		{name: "Active thread Registers", mut: func(e *ExpectedMTState, st *multithreaded.State) {
			e.threadExpectations[st.GetCurrentThread().ThreadId].Registers[0] += 1
		}},
		{name: "Active thread dropped", mut: func(e *ExpectedMTState, st *multithreaded.State) {
			e.threadExpectations[st.GetCurrentThread().ThreadId].Dropped = true
		}},
		{name: "Inactive threadId", mut: func(e *ExpectedMTState, st *multithreaded.State) {
			e.threadExpectations[FindNextThread(st).ThreadId].ThreadId += 1
		}},
		{name: "Inactive thread exitCode", mut: func(e *ExpectedMTState, st *multithreaded.State) {
			e.threadExpectations[FindNextThread(st).ThreadId].ExitCode += 1
		}},
		{name: "Inactive thread exited", mut: func(e *ExpectedMTState, st *multithreaded.State) {
			e.threadExpectations[FindNextThread(st).ThreadId].Exited = !FindNextThread(st).Exited
		}},
		{name: "Inactive thread futexAddr", mut: func(e *ExpectedMTState, st *multithreaded.State) {
			e.threadExpectations[FindNextThread(st).ThreadId].FutexAddr += 1
		}},
		{name: "Inactive thread futexVal", mut: func(e *ExpectedMTState, st *multithreaded.State) {
			e.threadExpectations[FindNextThread(st).ThreadId].FutexVal += 1
		}},
		{name: "Inactive thread FutexTimeoutStep", mut: func(e *ExpectedMTState, st *multithreaded.State) {
			e.threadExpectations[FindNextThread(st).ThreadId].FutexTimeoutStep += 1
		}},
		{name: "Inactive thread PC", mut: func(e *ExpectedMTState, st *multithreaded.State) {
			e.threadExpectations[FindNextThread(st).ThreadId].PC += 1
		}},
		{name: "Inactive thread NextPC", mut: func(e *ExpectedMTState, st *multithreaded.State) {
			e.threadExpectations[FindNextThread(st).ThreadId].NextPC += 1
		}},
		{name: "Inactive thread HI", mut: func(e *ExpectedMTState, st *multithreaded.State) {
			e.threadExpectations[FindNextThread(st).ThreadId].HI += 1
		}},
		{name: "Inactive thread LO", mut: func(e *ExpectedMTState, st *multithreaded.State) {
			e.threadExpectations[FindNextThread(st).ThreadId].LO += 1
		}},
		{name: "Inactive thread Registers", mut: func(e *ExpectedMTState, st *multithreaded.State) {
			e.threadExpectations[FindNextThread(st).ThreadId].Registers[0] += 1
		}},
		{name: "Inactive thread dropped", mut: func(e *ExpectedMTState, st *multithreaded.State) {
			e.threadExpectations[FindNextThread(st).ThreadId].Dropped = true
		}},
	}
	for _, c := range cases {
		for i, state := range states {
			testName := fmt.Sprintf("%v (state #%v)", c.name, i)
			t.Run(testName, func(t *testing.T) {
				expected := NewExpectedMTState(state)
				c.mut(expected, state)

				// We should detect the change and fail
				mockT := &MockTestingT{}
				expected.Validate(mockT, state)
				mockT.RequireFailed(t)
			})
		}

	}
}

func TestValidate_shouldPassUnchangedExpectations(t *testing.T) {
	states := []*multithreaded.State{
		RandomState(0),
		RandomState(1),
		RandomState(2),
	}

	for i, state := range states {
		testName := fmt.Sprintf("State #%v", i)
		t.Run(testName, func(t *testing.T) {
			expected := NewExpectedMTState(state)

			mockT := &MockTestingT{}
			expected.Validate(mockT, state)
			mockT.RequireNoFailure(t)
		})
	}
}

type MockTestingT struct {
	errCount int
}

var _ require.TestingT = (*MockTestingT)(nil)

func (m *MockTestingT) Errorf(format string, args ...interface{}) {
	m.errCount += 1
}

func (m *MockTestingT) FailNow() {
	m.errCount += 1
}

func (m *MockTestingT) RequireFailed(t require.TestingT) {
	require.Greater(t, m.errCount, 0, "Should have tracked a failure")
}

func (m *MockTestingT) RequireNoFailure(t require.TestingT) {
	require.Equal(t, m.errCount, 0, "Should not have tracked a failure")
}
