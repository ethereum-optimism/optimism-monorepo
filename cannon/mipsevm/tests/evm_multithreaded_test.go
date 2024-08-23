package tests

import (
	"fmt"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/cannon/mipsevm"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/exec"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/multithreaded"
	mttestutil "github.com/ethereum-optimism/optimism/cannon/mipsevm/multithreaded/testutil"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/testutil"
)

func TestEVM_SysClone_FlagHandling(t *testing.T) {
	contracts := testutil.TestContractsSetup(t, testutil.MipsMultithreaded)
	var tracer *tracing.Hooks

	cases := []struct {
		name  string
		flags uint32
		valid bool
	}{
		{"the supported flags bitmask", exec.ValidCloneFlags, true},
		{"no flags", 0, false},
		{"all flags", ^uint32(0), false},
		{"all unsupported flags", ^uint32(exec.ValidCloneFlags), false},
		{"a few supported flags", exec.CloneFs | exec.CloneSysvsem, false},
		{"one supported flag", exec.CloneFs, false},
		{"mixed supported and unsupported flags", exec.CloneFs | exec.CloneParentSettid, false},
		{"a single unsupported flag", exec.CloneUntraced, false},
		{"multiple unsupported flags", exec.CloneUntraced | exec.CloneParentSettid, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			state := multithreaded.CreateEmptyState()
			state.Memory.SetMemory(state.GetPC(), syscallInsn)
			state.GetRegistersRef()[2] = exec.SysClone // Set syscall number
			state.GetRegistersRef()[4] = c.flags       // Set first argument
			curStep := state.Step

			var err error
			var stepWitness *mipsevm.StepWitness
			us := multithreaded.NewInstrumentedState(state, nil, os.Stdout, os.Stderr, nil)
			if !c.valid {
				// The VM should exit
				stepWitness, err = us.Step(true)
				require.NoError(t, err)
				require.Equal(t, curStep+1, state.GetStep())
				require.Equal(t, true, us.GetState().GetExited())
				require.Equal(t, uint8(mipsevm.VMStatusPanic), us.GetState().GetExitCode())
				require.Equal(t, 1, state.ThreadCount())
			} else {
				stepWitness, err = us.Step(true)
				require.NoError(t, err)
				require.Equal(t, curStep+1, state.GetStep())
				require.Equal(t, false, us.GetState().GetExited())
				require.Equal(t, uint8(0), us.GetState().GetExitCode())
				require.Equal(t, 2, state.ThreadCount())
			}

			evm := testutil.NewMIPSEVM(contracts)
			evm.SetTracer(tracer)
			testutil.LogStepFailureAtCleanup(t, evm)

			evmPost := evm.Step(t, stepWitness, curStep, multithreaded.GetStateHashFn())
			goPost, _ := us.GetState().EncodeWitness()
			require.Equal(t, hexutil.Bytes(goPost).String(), hexutil.Bytes(evmPost).String(),
				"mipsevm produced different state than EVM")
		})
	}
}

func TestEVM_SysClone_Successful(t *testing.T) {
	var tracer *tracing.Hooks
	cases := []struct {
		name          string
		traverseRight bool
	}{
		{"traverse left", false},
		{"traverse right", true},
	}

	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			stackPtr := uint32(100)

			goVm, state, contracts := setup(t, i)
			mttestutil.InitializeSingleThread(i*333, state, c.traverseRight)
			state.Memory.SetMemory(state.GetPC(), syscallInsn)
			state.GetRegistersRef()[2] = exec.SysClone        // the syscall number
			state.GetRegistersRef()[4] = exec.ValidCloneFlags // a0 - first argument, clone flags
			state.GetRegistersRef()[5] = stackPtr             // a1 - the stack pointer
			curStep := state.GetStep()

			// Sanity-check assumptions
			require.Equal(t, uint32(1), state.NextThreadId)

			// Setup expectations
			expected := mttestutil.NewExpectedMTState(state)
			expected.Step += 1
			expectedNewThread := expected.ExpectNewThread()
			expected.ActiveThreadId = expectedNewThread.ThreadId
			expected.StepsSinceLastContextSwitch = 0
			if c.traverseRight {
				expected.RightStackSize += 1
			} else {
				expected.LeftStackSize += 1
			}
			// Original thread expectations
			expected.PrestateActiveThread().PC = state.GetCpu().NextPC
			expected.PrestateActiveThread().NextPC = state.GetCpu().NextPC + 4
			expected.PrestateActiveThread().Registers[2] = 1
			expected.PrestateActiveThread().Registers[7] = 0
			// New thread expectations
			expectedNewThread.PC = state.GetCpu().NextPC
			expectedNewThread.NextPC = state.GetCpu().NextPC + 4
			expectedNewThread.ThreadId = 1
			expectedNewThread.Registers[2] = 0
			expectedNewThread.Registers[7] = 0
			expectedNewThread.Registers[29] = stackPtr

			var err error
			var stepWitness *mipsevm.StepWitness
			stepWitness, err = goVm.Step(true)
			require.NoError(t, err)

			var activeStack, inactiveStack []*multithreaded.ThreadState
			if c.traverseRight {
				activeStack = state.RightThreadStack
				inactiveStack = state.LeftThreadStack
			} else {
				activeStack = state.LeftThreadStack
				inactiveStack = state.RightThreadStack
			}

			expected.Validate(t, state)
			require.Equal(t, 2, len(activeStack))
			require.Equal(t, 0, len(inactiveStack))

			evm := testutil.NewMIPSEVM(contracts)
			evm.SetTracer(tracer)
			testutil.LogStepFailureAtCleanup(t, evm)

			evmPost := evm.Step(t, stepWitness, curStep, multithreaded.GetStateHashFn())
			goPost, _ := goVm.GetState().EncodeWitness()
			require.Equal(t, hexutil.Bytes(goPost).String(), hexutil.Bytes(evmPost).String(),
				"mipsevm produced different state than EVM")
		})
	}
}

func TestEVM_SysGetTID(t *testing.T) {
	var tracer *tracing.Hooks
	cases := []struct {
		name     string
		threadId uint32
	}{
		{"zero", 0},
		{"non-zero", 11},
	}

	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			goVm, state, contracts := setup(t, i*789)
			mttestutil.InitializeSingleThread(i*789, state, false)

			state.GetCurrentThread().ThreadId = c.threadId
			state.Memory.SetMemory(state.GetPC(), syscallInsn)
			state.GetRegistersRef()[2] = exec.SysGetTID // Set syscall number
			step := state.Step

			// Set up post-state expectations
			expected := mttestutil.NewExpectedMTState(state)
			expected.ExpectStep()
			expected.ActiveThread().Registers[2] = c.threadId
			expected.ActiveThread().Registers[7] = 0

			// State transition
			var err error
			var stepWitness *mipsevm.StepWitness
			stepWitness, err = goVm.Step(true)
			require.NoError(t, err)

			// Validate post-state
			expected.Validate(t, state)

			evm := testutil.NewMIPSEVM(contracts)
			evm.SetTracer(tracer)
			testutil.LogStepFailureAtCleanup(t, evm)

			evmPost := evm.Step(t, stepWitness, step, multithreaded.GetStateHashFn())
			goPost, _ := goVm.GetState().EncodeWitness()
			require.Equal(t, hexutil.Bytes(goPost).String(), hexutil.Bytes(evmPost).String(),
				"mipsevm produced different state than EVM")
		})
	}
}

func TestEVM_SysExit(t *testing.T) {
	var tracer *tracing.Hooks
	cases := []struct {
		name               string
		threadCount        int
		shouldExitGlobally bool
	}{
		// If we exit the last thread, the whole process should exit
		{name: "one thread", threadCount: 1, shouldExitGlobally: true},
		{name: "two threads ", threadCount: 2},
		{name: "three threads ", threadCount: 3},
	}

	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			exitCode := uint8(3)

			goVm, state, contracts := setup(t, i*133)
			mttestutil.SetupThreads(int64(i*1111), state, i%2 == 0, c.threadCount, 0)

			state.Memory.SetMemory(state.GetPC(), syscallInsn)
			state.GetRegistersRef()[2] = exec.SysExit     // Set syscall number
			state.GetRegistersRef()[4] = uint32(exitCode) // The first argument (exit code)
			curStep := state.Step

			// Set up expectations
			expected := mttestutil.NewExpectedMTState(state)
			expected.Step += 1
			expected.StepsSinceLastContextSwitch += 1
			expected.ActiveThread().Exited = true
			expected.ActiveThread().ExitCode = exitCode
			if c.shouldExitGlobally {
				expected.Exited = true
				expected.ExitCode = exitCode
			}

			// State transition
			var err error
			var stepWitness *mipsevm.StepWitness
			stepWitness, err = goVm.Step(true)
			require.NoError(t, err)

			// Validate post-state
			expected.Validate(t, state)

			evm := testutil.NewMIPSEVM(contracts)
			evm.SetTracer(tracer)
			testutil.LogStepFailureAtCleanup(t, evm)

			evmPost := evm.Step(t, stepWitness, curStep, multithreaded.GetStateHashFn())
			goPost, _ := goVm.GetState().EncodeWitness()
			require.Equal(t, hexutil.Bytes(goPost).String(), hexutil.Bytes(evmPost).String(),
				"mipsevm produced different state than EVM")
		})
	}
}

func TestEVM_PopExitedThread(t *testing.T) {
	var tracer *tracing.Hooks
	cases := []struct {
		name                         string
		traverseRight                bool
		activeStackThreadCount       int
		expectTraverseRightPostState bool
	}{
		{name: "traverse right", traverseRight: true, activeStackThreadCount: 2, expectTraverseRightPostState: true},
		{name: "traverse right, switch directions", traverseRight: true, activeStackThreadCount: 1, expectTraverseRightPostState: false},
		{name: "traverse left", traverseRight: false, activeStackThreadCount: 2, expectTraverseRightPostState: false},
		{name: "traverse left, switch directions", traverseRight: false, activeStackThreadCount: 1, expectTraverseRightPostState: true},
	}

	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			goVm, state, contracts := setup(t, i*133)
			mttestutil.SetupThreads(int64(i*222), state, c.traverseRight, c.activeStackThreadCount, 1)
			step := state.Step

			// Setup thread to be dropped
			threadToPop := state.GetCurrentThread()
			threadToPop.Exited = true
			threadToPop.ExitCode = 1

			// Set up expectations
			expected := mttestutil.NewExpectedMTState(state)
			expected.Step += 1
			expected.ActiveThreadId = mttestutil.FindNextThreadExcluding(state, threadToPop.ThreadId).ThreadId
			expected.StepsSinceLastContextSwitch = 0
			expected.ThreadCount -= 1
			expected.TraverseRight = c.expectTraverseRightPostState
			expected.Thread(threadToPop.ThreadId).Dropped = true
			if c.traverseRight {
				expected.RightStackSize -= 1
			} else {
				expected.LeftStackSize -= 1
			}

			// State transition
			var err error
			var stepWitness *mipsevm.StepWitness
			stepWitness, err = goVm.Step(true)
			require.NoError(t, err)

			// Validate post-state
			expected.Validate(t, state)

			evm := testutil.NewMIPSEVM(contracts)
			evm.SetTracer(tracer)
			testutil.LogStepFailureAtCleanup(t, evm)

			evmPost := evm.Step(t, stepWitness, step, multithreaded.GetStateHashFn())
			goPost, _ := goVm.GetState().EncodeWitness()
			require.Equal(t, hexutil.Bytes(goPost).String(), hexutil.Bytes(evmPost).String(),
				"mipsevm produced different state than EVM")
		})
	}
}

func TestEVM_SysFutex_WaitPrivate(t *testing.T) {
	var tracer *tracing.Hooks
	cases := []struct {
		name             string
		address          uint32
		targetValue      uint32
		actualValue      uint32
		timeout          uint32
		shouldFail       bool
		shouldSetTimeout bool
	}{
		{name: "successful wait, no timeout", address: 0x1234, targetValue: 0x01, actualValue: 0x01},
		{name: "memory mismatch, no timeout", address: 0x1200, targetValue: 0x01, actualValue: 0x02, shouldFail: true},
		{name: "successful wait w timeout", address: 0x1234, targetValue: 0x01, actualValue: 0x01, timeout: 1000000, shouldSetTimeout: true},
		{name: "memory mismatch w timeout", address: 0x1200, targetValue: 0x01, actualValue: 0x02, timeout: 2000000, shouldFail: true},
	}

	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			goVm, state, contracts := setup(t, i*1234)
			step := state.GetStep()

			state.Memory.SetMemory(state.GetPC(), syscallInsn)
			state.Memory.SetMemory(c.address, c.actualValue)
			state.GetRegistersRef()[2] = exec.SysFutex // Set syscall number
			state.GetRegistersRef()[4] = c.address
			state.GetRegistersRef()[5] = exec.FutexWaitPrivate
			state.GetRegistersRef()[6] = c.targetValue
			state.GetRegistersRef()[7] = c.timeout

			// Setup expectations
			expected := mttestutil.NewExpectedMTState(state)
			expected.Step += 1
			expected.StepsSinceLastContextSwitch += 1
			if c.shouldFail {
				expected.ActiveThread().PC = state.GetCpu().NextPC
				expected.ActiveThread().NextPC = state.GetCpu().NextPC + 4
				expected.ActiveThread().Registers[2] = exec.SysErrorSignal
				expected.ActiveThread().Registers[7] = exec.MipsEAGAIN
			} else {
				// PC and return registers should not update on success, updates happen when wait completes
				expected.ActiveThread().FutexAddr = c.address
				expected.ActiveThread().FutexVal = c.targetValue
				expected.ActiveThread().FutexTimeoutStep = exec.FutexNoTimeout
				if c.shouldSetTimeout {
					expected.ActiveThread().FutexTimeoutStep = step + exec.FutexTimeoutSteps + 1
				}
			}

			// State transition
			var err error
			var stepWitness *mipsevm.StepWitness
			stepWitness, err = goVm.Step(true)
			require.NoError(t, err)

			// Validate post-state
			expected.Validate(t, state)

			evm := testutil.NewMIPSEVM(contracts)
			evm.SetTracer(tracer)
			testutil.LogStepFailureAtCleanup(t, evm)

			evmPost := evm.Step(t, stepWitness, step, multithreaded.GetStateHashFn())
			goPost, _ := goVm.GetState().EncodeWitness()
			require.Equal(t, hexutil.Bytes(goPost).String(), hexutil.Bytes(evmPost).String(),
				"mipsevm produced different state than EVM")
		})
	}
}

func TestEVM_HandleWaitingThread_NormalTraversal(t *testing.T) {
	var tracer *tracing.Hooks
	cases := []struct {
		name            string
		step            uint64
		activeStackSize int
		otherStackSize  int
		futexAddr       uint32
		targetValue     uint32
		actualValue     uint32
		timeoutStep     uint64
		shouldWakeup    bool
		shouldTimeout   bool
	}{
		{name: "Preempt, no timeout #1", step: 100, activeStackSize: 1, otherStackSize: 0, futexAddr: 0x100, targetValue: 0x01, actualValue: 0x01, timeoutStep: exec.FutexNoTimeout},
		{name: "Preempt, no timeout #2", step: 100, activeStackSize: 1, otherStackSize: 1, futexAddr: 0x100, targetValue: 0x01, actualValue: 0x01, timeoutStep: exec.FutexNoTimeout},
		{name: "Preempt, no timeout #3", step: 100, activeStackSize: 2, otherStackSize: 1, futexAddr: 0x100, targetValue: 0x01, actualValue: 0x01, timeoutStep: exec.FutexNoTimeout},
		{name: "Preempt, with timeout #1", step: 100, activeStackSize: 2, otherStackSize: 1, futexAddr: 0x100, targetValue: 0x01, actualValue: 0x01, timeoutStep: 101},
		{name: "Preempt, with timeout #2", step: 100, activeStackSize: 1, otherStackSize: 1, futexAddr: 0x100, targetValue: 0x01, actualValue: 0x01, timeoutStep: 150},
		{name: "Wakeup, no timeout #1", step: 100, activeStackSize: 1, otherStackSize: 0, futexAddr: 0x100, targetValue: 0x01, actualValue: 0x02, timeoutStep: exec.FutexNoTimeout, shouldWakeup: true},
		{name: "Wakeup, no timeout #2", step: 100, activeStackSize: 2, otherStackSize: 1, futexAddr: 0x100, targetValue: 0x01, actualValue: 0x02, timeoutStep: exec.FutexNoTimeout, shouldWakeup: true},
		{name: "Wakeup with timeout #1", step: 100, activeStackSize: 2, otherStackSize: 1, futexAddr: 0x100, targetValue: 0x01, actualValue: 0x02, timeoutStep: 100, shouldWakeup: true, shouldTimeout: true},
		{name: "Wakeup with timeout #2", step: 100, activeStackSize: 2, otherStackSize: 1, futexAddr: 0x100, targetValue: 0x02, actualValue: 0x02, timeoutStep: 100, shouldWakeup: true, shouldTimeout: true},
		{name: "Wakeup with timeout #3", step: 100, activeStackSize: 2, otherStackSize: 1, futexAddr: 0x100, targetValue: 0x02, actualValue: 0x02, timeoutStep: 50, shouldWakeup: true, shouldTimeout: true},
	}

	for _, c := range cases {
		for i, traverseRight := range []bool{true, false} {
			testName := fmt.Sprintf("%v (traverseRight=%v)", c.name, traverseRight)
			t.Run(testName, func(t *testing.T) {
				// Sanity check
				if !c.shouldWakeup && c.shouldTimeout {
					require.Fail(t, "Invalid test case - cannot expect a timeout with no wakeup")
				}

				goVm, state, contracts := setup(t, i)
				mttestutil.SetupThreads(int64(i*101), state, traverseRight, c.activeStackSize, c.otherStackSize)
				state.Step = c.step

				activeThread := state.GetCurrentThread()
				activeThread.FutexAddr = c.futexAddr
				activeThread.FutexVal = c.targetValue
				activeThread.FutexTimeoutStep = c.timeoutStep
				state.GetMemory().SetMemory(c.futexAddr, c.actualValue)

				// Set up post-state expectations
				expected := mttestutil.NewExpectedMTState(state)
				expected.Step += 1
				if c.shouldWakeup {
					expected.ActiveThread().FutexAddr = exec.FutexEmptyAddr
					expected.ActiveThread().FutexVal = 0
					expected.ActiveThread().FutexTimeoutStep = 0
					// PC and return registers are updated onWaitComplete
					expected.ActiveThread().PC = state.GetCpu().NextPC
					expected.ActiveThread().NextPC = state.GetCpu().NextPC + 4
					if c.shouldTimeout {
						expected.ActiveThread().Registers[2] = exec.SysErrorSignal
						expected.ActiveThread().Registers[7] = exec.MipsETIMEDOUT
					} else {
						expected.ActiveThread().Registers[2] = 0
						expected.ActiveThread().Registers[7] = 0
					}
				} else {
					expected.ExpectPreemption(state)
				}

				// State transition
				var err error
				var stepWitness *mipsevm.StepWitness
				stepWitness, err = goVm.Step(true)
				require.NoError(t, err)

				// Validate post-state
				expected.Validate(t, state)

				evm := testutil.NewMIPSEVM(contracts)
				evm.SetTracer(tracer)
				testutil.LogStepFailureAtCleanup(t, evm)

				evmPost := evm.Step(t, stepWitness, c.step, multithreaded.GetStateHashFn())
				goPost, _ := goVm.GetState().EncodeWitness()
				require.Equal(t, hexutil.Bytes(goPost).String(), hexutil.Bytes(evmPost).String(),
					"mipsevm produced different state than EVM")
			})

		}
	}
}

func TestEVM_SysFutex_WakePrivate(t *testing.T) {
	var tracer *tracing.Hooks
	cases := []struct {
		name                   string
		address                uint32
		activeThreadCount      int
		inactiveThreadCount    int
		traverseRight          bool
		expectTraverseRight    bool
		expectSameActiveThread bool
	}{
		{name: "Traverse right", address: 0x6789, activeThreadCount: 2, inactiveThreadCount: 1, traverseRight: true, expectSameActiveThread: true},
		{name: "Traverse right, no left threads", address: 0x6789, activeThreadCount: 2, inactiveThreadCount: 0, traverseRight: true, expectSameActiveThread: true},
		{name: "Traverse left", address: 0x6789, activeThreadCount: 2, inactiveThreadCount: 1, traverseRight: false},
		{name: "Traverse left, switch directions", address: 0x6789, activeThreadCount: 1, inactiveThreadCount: 1, traverseRight: false, expectTraverseRight: true, expectSameActiveThread: true},
	}

	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			goVm, state, contracts := setup(t, i*1122)
			mttestutil.SetupThreads(int64(i*2244), state, c.traverseRight, c.activeThreadCount, c.inactiveThreadCount)
			step := state.Step

			state.Memory.SetMemory(state.GetPC(), syscallInsn)
			state.GetRegistersRef()[2] = exec.SysFutex // Set syscall number
			state.GetRegistersRef()[4] = c.address
			state.GetRegistersRef()[5] = exec.FutexWakePrivate

			// Set up post-state expectations
			expected := mttestutil.NewExpectedMTState(state)
			expected.ExpectStep()
			expected.ActiveThread().Registers[2] = 0
			expected.ActiveThread().Registers[7] = 0
			expected.Wakeup = c.address
			expected.ExpectPreemption(state)
			expected.TraverseRight = c.expectTraverseRight
			if c.expectSameActiveThread {
				// The preemption logic for this syscall is unusual because we do a normal preemption, then
				// switch to traversing left when possible.  So we can, for example, preempt a thread moving it
				// from the right stack to the left and then visit the same thread again on the left stack
				expected.ActiveThreadId = state.GetCurrentThread().ThreadId
			}

			// State transition
			var err error
			var stepWitness *mipsevm.StepWitness
			stepWitness, err = goVm.Step(true)
			require.NoError(t, err)

			// Validate post-state
			expected.Validate(t, state)

			evm := testutil.NewMIPSEVM(contracts)
			evm.SetTracer(tracer)
			testutil.LogStepFailureAtCleanup(t, evm)

			evmPost := evm.Step(t, stepWitness, step, multithreaded.GetStateHashFn())
			goPost, _ := goVm.GetState().EncodeWitness()
			require.Equal(t, hexutil.Bytes(goPost).String(), hexutil.Bytes(evmPost).String(),
				"mipsevm produced different state than EVM")
		})
	}
}

func TestEVM_WakeupTraversalStep(t *testing.T) {
	wakeupAddr := uint32(0x1234)
	wakeupVal := uint32(0x999)
	var tracer *tracing.Hooks
	cases := []struct {
		name              string
		futexAddr         uint32
		targetVal         uint32
		traverseRight     bool
		activeStackSize   int
		otherStackSize    int
		shouldClearWakeup bool
		shouldWakeThread  bool
	}{
		{name: "Find match, preempt first thread", futexAddr: wakeupAddr, targetVal: wakeupVal, traverseRight: false, activeStackSize: 3, otherStackSize: 0, shouldClearWakeup: true},
		{name: "Find match, wake first thread", futexAddr: wakeupAddr, targetVal: wakeupVal + 1, traverseRight: false, activeStackSize: 3, otherStackSize: 0, shouldClearWakeup: true, shouldWakeThread: true},
		{name: "Find match, preempt last thread", futexAddr: wakeupAddr, targetVal: wakeupVal, traverseRight: true, activeStackSize: 1, otherStackSize: 2, shouldClearWakeup: true},
		{name: "Find match, wake last thread", futexAddr: wakeupAddr, targetVal: wakeupVal + 1, traverseRight: true, activeStackSize: 1, otherStackSize: 2, shouldClearWakeup: true, shouldWakeThread: true},
		{name: "Find match, preempt intermediate thread", futexAddr: wakeupAddr, targetVal: wakeupVal, traverseRight: false, activeStackSize: 2, otherStackSize: 2, shouldClearWakeup: true},
		{name: "Find match, wake intermediate thread", futexAddr: wakeupAddr, targetVal: wakeupVal + 1, traverseRight: true, activeStackSize: 2, otherStackSize: 2, shouldClearWakeup: true, shouldWakeThread: true},
		{name: "Complete traversal without match", futexAddr: wakeupAddr + 4, traverseRight: true, activeStackSize: 1, otherStackSize: 2, shouldClearWakeup: true},
		{name: "Continue traversal", futexAddr: wakeupAddr + 4, traverseRight: true, activeStackSize: 2, otherStackSize: 2, shouldClearWakeup: false},
		{name: "Continue traversal", futexAddr: wakeupAddr + 4, traverseRight: false, activeStackSize: 2, otherStackSize: 0, shouldClearWakeup: false},
		{name: "Continue traversal", futexAddr: wakeupAddr + 4, traverseRight: false, activeStackSize: 1, otherStackSize: 0, shouldClearWakeup: false},
	}

	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			goVm, state, contracts := setup(t, i*2000)
			mttestutil.SetupThreads(int64(i*101), state, c.traverseRight, c.activeStackSize, c.otherStackSize)
			step := state.Step

			activeThread := state.GetCurrentThread()
			activeThread.FutexAddr = c.futexAddr
			activeThread.FutexVal = c.targetVal
			activeThread.FutexTimeoutStep = exec.FutexNoTimeout
			state.GetMemory().SetMemory(wakeupAddr, wakeupVal)

			// Set up post-state expectations
			expected := mttestutil.NewExpectedMTState(state)
			expected.Step += 1
			if c.shouldClearWakeup {
				expected.Wakeup = exec.FutexEmptyAddr
				if c.shouldWakeThread {
					expected.ActiveThread().PC = state.GetCpu().NextPC
					expected.ActiveThread().NextPC = state.GetCpu().NextPC + 4
					expected.ActiveThread().FutexAddr = exec.FutexEmptyAddr
					expected.ActiveThread().FutexVal = 0
					expected.ActiveThread().FutexTimeoutStep = 0
					expected.ActiveThread().Registers[2] = 0
					expected.ActiveThread().Registers[7] = 0
				} else {
					expected.ExpectPreemption(state)
				}
			} else {
				// Just preempt the curren thread
				expected.ExpectPreemption(state)
			}

			// State transition
			var err error
			var stepWitness *mipsevm.StepWitness
			stepWitness, err = goVm.Step(true)
			require.NoError(t, err)

			// Validate post-state
			expected.Validate(t, state)

			evm := testutil.NewMIPSEVM(contracts)
			evm.SetTracer(tracer)
			testutil.LogStepFailureAtCleanup(t, evm)

			evmPost := evm.Step(t, stepWitness, step, multithreaded.GetStateHashFn())
			goPost, _ := goVm.GetState().EncodeWitness()
			require.Equal(t, hexutil.Bytes(goPost).String(), hexutil.Bytes(evmPost).String(),
				"mipsevm produced different state than EVM")
		})
	}
}

func TestEVM_SysFutex_UnsupportedOp(t *testing.T) {
	t.Skip("TODO")
}

func TestEVM_SysYield(t *testing.T) {
	var tracer *tracing.Hooks
	cases := []struct {
		name            string
		traverseRight   bool
		activeThreads   int
		inactiveThreads int
	}{
		{name: "Last active thread", activeThreads: 1, inactiveThreads: 2},
		{name: "Only thread", activeThreads: 1, inactiveThreads: 0},
		{name: "Do not change directions", activeThreads: 2, inactiveThreads: 2},
		{name: "Do not change directions", activeThreads: 3, inactiveThreads: 0},
	}

	for i, c := range cases {
		for _, traverseRight := range []bool{true, false} {
			testName := fmt.Sprintf("%v (traverseRight = %v)", c.name, traverseRight)
			t.Run(testName, func(t *testing.T) {
				goVm, state, contracts := setup(t, i*789)
				mttestutil.SetupThreads(int64(i*3259), state, traverseRight, c.activeThreads, c.inactiveThreads)

				state.Memory.SetMemory(state.GetPC(), syscallInsn)
				state.GetRegistersRef()[2] = exec.SysSchedYield // Set syscall number
				step := state.Step

				// Set up post-state expectations
				expected := mttestutil.NewExpectedMTState(state)
				expected.ExpectStep()
				expected.ExpectPreemption(state)
				expected.PrestateActiveThread().Registers[2] = 0
				expected.PrestateActiveThread().Registers[7] = 0

				// State transition
				var err error
				var stepWitness *mipsevm.StepWitness
				stepWitness, err = goVm.Step(true)
				require.NoError(t, err)

				// Validate post-state
				expected.Validate(t, state)

				evm := testutil.NewMIPSEVM(contracts)
				evm.SetTracer(tracer)
				testutil.LogStepFailureAtCleanup(t, evm)

				evmPost := evm.Step(t, stepWitness, step, multithreaded.GetStateHashFn())
				goPost, _ := goVm.GetState().EncodeWitness()
				require.Equal(t, hexutil.Bytes(goPost).String(), hexutil.Bytes(evmPost).String(),
					"mipsevm produced different state than EVM")
			})
		}
	}
}

func TestEVM_SysOpen(t *testing.T) {
	var tracer *tracing.Hooks

	goVm, state, contracts := setup(t, 5512)

	state.Memory.SetMemory(state.GetPC(), syscallInsn)
	state.GetRegistersRef()[2] = exec.SysOpen // Set syscall number
	step := state.Step

	// Set up post-state expectations
	expected := mttestutil.NewExpectedMTState(state)
	expected.ExpectStep()
	expected.ActiveThread().Registers[2] = exec.SysErrorSignal
	expected.ActiveThread().Registers[7] = exec.MipsEBADF

	// State transition
	var err error
	var stepWitness *mipsevm.StepWitness
	stepWitness, err = goVm.Step(true)
	require.NoError(t, err)

	// Validate post-state
	expected.Validate(t, state)

	evm := testutil.NewMIPSEVM(contracts)
	evm.SetTracer(tracer)
	testutil.LogStepFailureAtCleanup(t, evm)

	evmPost := evm.Step(t, stepWitness, step, multithreaded.GetStateHashFn())
	goPost, _ := goVm.GetState().EncodeWitness()
	require.Equal(t, hexutil.Bytes(goPost).String(), hexutil.Bytes(evmPost).String(),
		"mipsevm produced different state than EVM")
}

func TestEVM_SchedQuantumThreshold(t *testing.T) {
	var tracer *tracing.Hooks
	cases := []struct {
		name                        string
		stepsSinceLastContextSwitch uint64
		shouldPreempt               bool
	}{
		{name: "just under threshold", stepsSinceLastContextSwitch: exec.SchedQuantum - 1},
		{name: "at threshold", stepsSinceLastContextSwitch: exec.SchedQuantum, shouldPreempt: true},
		{name: "beyond threshold", stepsSinceLastContextSwitch: exec.SchedQuantum + 1, shouldPreempt: true},
	}

	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			goVm, state, contracts := setup(t, i*789)
			// Setup basic getThreadId syscall instruction
			state.Memory.SetMemory(state.GetPC(), syscallInsn)
			state.GetRegistersRef()[2] = exec.SysGetTID // Set syscall number
			state.StepsSinceLastContextSwitch = c.stepsSinceLastContextSwitch
			step := state.Step

			// Set up post-state expectations
			expected := mttestutil.NewExpectedMTState(state)
			if c.shouldPreempt {
				expected.Step += 1
				expected.ExpectPreemption(state)
			} else {
				expected.ExpectStep()
				expected.ActiveThread().Registers[2] = state.GetCurrentThread().ThreadId
				expected.ActiveThread().Registers[7] = 0
			}

			// State transition
			var err error
			var stepWitness *mipsevm.StepWitness
			stepWitness, err = goVm.Step(true)
			require.NoError(t, err)

			// Validate post-state
			expected.Validate(t, state)

			evm := testutil.NewMIPSEVM(contracts)
			evm.SetTracer(tracer)
			testutil.LogStepFailureAtCleanup(t, evm)

			evmPost := evm.Step(t, stepWitness, step, multithreaded.GetStateHashFn())
			goPost, _ := goVm.GetState().EncodeWitness()
			require.Equal(t, hexutil.Bytes(goPost).String(), hexutil.Bytes(evmPost).String(),
				"mipsevm produced different state than EVM")
		})
	}
}

func TestEVM_NoopInstruction(t *testing.T) {
	t.Skip("TODO")
}

func TestEVM_UnsupportedSyscall(t *testing.T) {
	t.Skip("TODO")
}

func setup(t require.TestingT, randomSeed int) (mipsevm.FPVM, *multithreaded.State, *testutil.ContractMetadata) {
	v := GetMultiThreadedTestCase(t)
	vm := v.VMFactory(nil, os.Stdout, os.Stderr, testutil.CreateLogger(), testutil.WithRandomization(int64(randomSeed)))
	state := mttestutil.GetMtState(t, vm)

	return vm, state, v.Contracts

}
