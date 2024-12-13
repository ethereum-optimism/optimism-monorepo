package testutil

import (
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/arch"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/multithreaded"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/testutil"
)

func RandomThread(randSeed int64) *multithreaded.ThreadState {
	r := testutil.NewRandHelper(randSeed)
	thread := multithreaded.CreateEmptyThread()

	pc := r.RandPC()

	thread.Registers = *r.RandRegisters()
	thread.Cpu.PC = pc
	thread.Cpu.NextPC = pc + 4
	thread.Cpu.HI = r.Word()
	thread.Cpu.LO = r.Word()

	return thread
}

func InitializeSingleThread(randSeed int, state *multithreaded.State, traverseRight bool, opts ...testutil.StateOption) {
	singleThread := RandomThread(int64(randSeed))

	state.NextThreadId = singleThread.ThreadId + 1
	state.TraverseRight = traverseRight
	if traverseRight {
		state.RightThreadStack = []*multithreaded.ThreadState{singleThread}
		state.LeftThreadStack = []*multithreaded.ThreadState{}
	} else {
		state.RightThreadStack = []*multithreaded.ThreadState{}
		state.LeftThreadStack = []*multithreaded.ThreadState{singleThread}
	}

	mutator := NewStateMutatorMultiThreaded(state)
	for _, opt := range opts {
		opt(mutator)
	}
}

func SetupThreads(randomSeed int64, state *multithreaded.State, traverseRight bool, activeStackSize, otherStackSize int) {
	var activeStack, otherStack []*multithreaded.ThreadState

	tid := arch.Word(0)
	for i := 0; i < activeStackSize; i++ {
		thread := RandomThread(randomSeed + int64(i))
		thread.ThreadId = tid
		activeStack = append(activeStack, thread)
		tid++
	}

	for i := 0; i < otherStackSize; i++ {
		thread := RandomThread(randomSeed + int64(i+activeStackSize))
		thread.ThreadId = tid
		otherStack = append(otherStack, thread)
		tid++
	}

	state.NextThreadId = tid
	state.TraverseRight = traverseRight
	if traverseRight {
		state.RightThreadStack = activeStack
		state.LeftThreadStack = otherStack
	} else {
		state.LeftThreadStack = activeStack
		state.RightThreadStack = otherStack
	}
}

type ThreadIterator struct {
	left          []*multithreaded.ThreadState
	right         []*multithreaded.ThreadState
	traverseRight bool
}

func NewThreadIterator(state *multithreaded.State) ThreadIterator {
	return ThreadIterator{
		left:          state.LeftThreadStack,
		right:         state.RightThreadStack,
		traverseRight: state.TraverseRight,
	}
}

func (i *ThreadIterator) currentThread() *multithreaded.ThreadState {
	var currentThread *multithreaded.ThreadState
	if i.traverseRight {
		currentThread = i.right[len(i.right)-1]
	} else {
		currentThread = i.left[len(i.left)-1]
	}
	return currentThread
}

func (i *ThreadIterator) Next() *multithreaded.ThreadState {
	rightLen := len(i.right)
	leftLen := len(i.left)
	activeThread := i.currentThread()

	if i.traverseRight {
		i.right = i.right[:rightLen-1]
		i.left = append(i.left, activeThread)
		i.traverseRight = len(i.right) > 0
	} else {
		i.left = i.left[:leftLen-1]
		i.right = append(i.right, activeThread)
		i.traverseRight = len(i.left) == 0
	}

	return i.currentThread()
}

// FindNextThread Finds the next thread in line according to thread traversal logic
func FindNextThread(state *multithreaded.State) *multithreaded.ThreadState {
	it := NewThreadIterator(state)
	return it.Next()
}

type ThreadFilter func(thread *multithreaded.ThreadState) bool

func FindNextThreadFiltered(state *multithreaded.State, filter ThreadFilter) *multithreaded.ThreadState {
	it := NewThreadIterator(state)

	// Worst case - walk all the way left, then all the way back right
	// Example w 3 threads: 1,2,3,3,2,1,0 -> 7 steps to find thread 0
	maxIterations := state.ThreadCount()*2 + 1
	for i := 0; i < maxIterations; i++ {
		next := it.Next()
		if filter(next) {
			return next
		}
	}

	return nil
}

func FindNextThreadExcluding(state *multithreaded.State, threadId arch.Word) *multithreaded.ThreadState {
	return FindNextThreadFiltered(state, func(t *multithreaded.ThreadState) bool {
		return t.ThreadId != threadId
	})
}

func FindThread(state *multithreaded.State, threadId arch.Word) *multithreaded.ThreadState {
	for _, t := range GetAllThreads(state) {
		if t.ThreadId == threadId {
			return t
		}
	}
	return nil
}

func GetAllThreads(state *multithreaded.State) []*multithreaded.ThreadState {
	allThreads := make([]*multithreaded.ThreadState, 0, state.ThreadCount())
	allThreads = append(allThreads, state.RightThreadStack[:]...)
	allThreads = append(allThreads, state.LeftThreadStack[:]...)

	return allThreads
}

func GetThreadStacks(state *multithreaded.State) (activeStack, inactiveStack []*multithreaded.ThreadState) {
	if state.TraverseRight {
		activeStack = state.RightThreadStack
		inactiveStack = state.LeftThreadStack
	} else {
		activeStack = state.LeftThreadStack
		inactiveStack = state.RightThreadStack
	}
	return activeStack, inactiveStack
}
