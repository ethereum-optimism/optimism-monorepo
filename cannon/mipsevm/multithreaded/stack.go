package multithreaded

import (
	"errors"

	"github.com/ethereum-optimism/optimism/cannon/mipsevm"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/exec"
)

type ThreadedStackTracker interface {
	exec.TraceableStackTracker
	DropThread(threadId Word)
}

type NoopThreadedStackTracker struct {
	exec.NoopStackTracker
}

var _ ThreadedStackTracker = (*ThreadedStackTrackerImpl)(nil)

func (n *NoopThreadedStackTracker) DropThread(threadId Word) {}

type ThreadedStackTrackerImpl struct {
	meta               mipsevm.Metadata
	state              *State
	trackersByThreadId map[Word]exec.TraceableStackTracker
}

var _ ThreadedStackTracker = (*ThreadedStackTrackerImpl)(nil)

func NewThreadedStackTracker(state *State, meta mipsevm.Metadata) (*ThreadedStackTrackerImpl, error) {
	if meta == nil {
		return nil, errors.New("metadata is nil")
	}

	return &ThreadedStackTrackerImpl{
		state:              state,
		meta:               meta,
		trackersByThreadId: make(map[Word]exec.TraceableStackTracker),
	}, nil
}

func (t *ThreadedStackTrackerImpl) PushStack(caller Word, target Word) {
	t.getCurrentTracker().PushStack(caller, target)
}

func (t *ThreadedStackTrackerImpl) PopStack() {
	t.getCurrentTracker().PopStack()
}

func (t *ThreadedStackTrackerImpl) Traceback() {
	t.getCurrentTracker().Traceback()
}

func (t *ThreadedStackTrackerImpl) getCurrentTracker() exec.TraceableStackTracker {
	thread := t.state.GetCurrentThread()
	tracker, exists := t.trackersByThreadId[thread.ThreadId]
	if !exists {
		tracker = exec.NewStackTrackerUnsafe(t.state, t.meta)
		t.trackersByThreadId[thread.ThreadId] = tracker
	}
	return tracker
}

func (t *ThreadedStackTrackerImpl) DropThread(threadId Word) {
	delete(t.trackersByThreadId, threadId)
}
