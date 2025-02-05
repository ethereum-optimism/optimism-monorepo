package exec

import (
	"errors"
	"fmt"

	"github.com/ethereum-optimism/optimism/cannon/mipsevm"
)

type StackTracker interface {
	PushStack(caller Word, target Word)
	PopStack()
}

type TraceableStackTracker interface {
	StackTracker
	Traceback()
}

type NoopStackTracker struct{}

func (n *NoopStackTracker) PushStack(caller Word, target Word) {}

func (n *NoopStackTracker) PopStack() {}

func (n *NoopStackTracker) Traceback() {}

type StackTrackerImpl struct {
	state mipsevm.FPVMState

	stack  []Word
	caller []Word
	meta   mipsevm.Metadata
}

func NewStackTracker(state mipsevm.FPVMState, meta mipsevm.Metadata) (*StackTrackerImpl, error) {
	if meta == nil {
		return nil, errors.New("metadata is nil")
	}
	return NewStackTrackerUnsafe(state, meta), nil
}

// NewStackTrackerUnsafe creates a new TraceableStackTracker without verifying meta is not nil
func NewStackTrackerUnsafe(state mipsevm.FPVMState, meta mipsevm.Metadata) *StackTrackerImpl {
	return &StackTrackerImpl{state: state, meta: meta}
}

func (s *StackTrackerImpl) PushStack(caller Word, target Word) {
	s.caller = append(s.caller, caller)
	s.stack = append(s.stack, target)
}

func (s *StackTrackerImpl) PopStack() {
	if len(s.stack) != 0 {
		fn := s.meta.LookupSymbol(s.state.GetPC())
		topFn := s.meta.LookupSymbol(s.stack[len(s.stack)-1])
		if fn != topFn {
			// most likely the function was inlined. Snap back to the last return.
			i := len(s.stack) - 1
			for ; i >= 0; i-- {
				if s.meta.LookupSymbol(s.stack[i]) == fn {
					s.stack = s.stack[:i]
					s.caller = s.caller[:i]
					break
				}
			}
		} else {
			s.stack = s.stack[:len(s.stack)-1]
			s.caller = s.caller[:len(s.caller)-1]
		}
	} else {
		fmt.Printf("ERROR: stack underflow at pc=%x. step=%d\n", s.state.GetPC(), s.state.GetStep())
	}
}

func (s *StackTrackerImpl) Traceback() {
	fmt.Printf("traceback at pc=%x. step=%d\n", s.state.GetPC(), s.state.GetStep())
	for i := len(s.stack) - 1; i >= 0; i-- {
		jumpAddr := s.stack[i]
		idx := len(s.stack) - i - 1
		fmt.Printf("\t%d %x in %s caller=%08x\n", idx, jumpAddr, s.meta.LookupSymbol(jumpAddr), s.caller[i])
	}
}
