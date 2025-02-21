package event

import (
	"context"
	"fmt"
	"io"
	"slices"
	"sync"
	"sync/atomic"
)

// Don't queue up an endless number of events.
// At some point it's better to drop events and warn something is exploding the number of events.
const sanityEventLimit = 10_000

type GlobalSyncExec struct {
	eventsLock sync.Mutex
	events     []AnnotatedEvent

	handles     []*globalHandle
	handlesLock sync.RWMutex

	ctx context.Context
}

var _ Executor = (*GlobalSyncExec)(nil)

func NewGlobalSynchronous(ctx context.Context) *GlobalSyncExec {
	return &GlobalSyncExec{ctx: ctx}
}

func (gs *GlobalSyncExec) Add(d Executable, _ *ExecutorOpts) (leaveExecutor func()) {
	gs.handlesLock.Lock()
	defer gs.handlesLock.Unlock()
	h := &globalHandle{d: d}
	h.g.Store(gs)
	gs.handles = append(gs.handles, h)
	return h.leave
}

func (gs *GlobalSyncExec) remove(h *globalHandle) {
	gs.handlesLock.Lock()
	defer gs.handlesLock.Unlock()
	// Linear search to delete is fine,
	// since we delete much less frequently than we process events with these.
	for i, v := range gs.handles {
		if v == h {
			gs.handles = slices.Delete(gs.handles, i, i+1)
			return
		}
	}
}

func (gs *GlobalSyncExec) Enqueue(ev AnnotatedEvent) error {
	gs.eventsLock.Lock()
	defer gs.eventsLock.Unlock()
	// sanity limit, never queue too many events
	if len(gs.events) >= sanityEventLimit {
		return fmt.Errorf("something is very wrong, queued up too many events! Dropping event %q", ev)
	}
	gs.events = append(gs.events, ev)
	return nil
}

func (gs *GlobalSyncExec) pop() AnnotatedEvent {
	gs.eventsLock.Lock()
	defer gs.eventsLock.Unlock()

	if len(gs.events) == 0 {
		return AnnotatedEvent{}
	}

	first := gs.events[0]
	gs.events = gs.events[1:]
	return first
}

func (gs *GlobalSyncExec) processEvent(ev AnnotatedEvent) {
	gs.handlesLock.RLock() // read lock, to allow Drain() to be called during event processing.
	defer gs.handlesLock.RUnlock()
	for _, h := range gs.handles {
		h.onEvent(ev)
	}
}

func (gs *GlobalSyncExec) Drain() error {
	for {
		if gs.ctx.Err() != nil {
			return gs.ctx.Err()
		}
		ev := gs.pop()
		if ev.Event == nil {
			return nil
		}
		// Note: event execution may call Drain(), that is allowed.
		gs.processEvent(ev)
	}
}

func (gs *GlobalSyncExec) DrainUntil(fn func(ev Event) bool, excl bool) error {
	// In order of operation:
	// stopExcl: stop draining, and leave the event.
	// no stopExcl, and no event: EOF, exhausted events before condition hit.
	// no stopExcl, and event: process event.
	// stopIncl: stop draining, after having processed the event first.
	iter := func() (ev AnnotatedEvent, stopIncl bool, stopExcl bool) {
		gs.eventsLock.Lock()
		defer gs.eventsLock.Unlock()

		if len(gs.events) == 0 {
			return AnnotatedEvent{}, false, false
		}

		ev = gs.events[0]
		stop := fn(ev.Event)
		if excl && stop {
			ev = AnnotatedEvent{}
			stopExcl = true
		} else {
			gs.events = gs.events[1:]
		}
		if stop {
			stopIncl = true
		}
		return
	}

	for {
		if gs.ctx.Err() != nil {
			return gs.ctx.Err()
		}
		// includes popping of the event, so we can handle Drain() calls by onEvent() execution
		ev, stopIncl, stopExcl := iter()
		if stopExcl {
			return nil
		}
		if ev.Event == nil {
			return io.EOF
		}
		gs.processEvent(ev)
		if stopIncl {
			return nil
		}
	}
}

type globalHandle struct {
	g atomic.Pointer[GlobalSyncExec]
	d Executable
}

func (gh *globalHandle) onEvent(ev AnnotatedEvent) {
	if gh.g.Load() == nil { // don't process more events while we are being removed
		return
	}
	gh.d.RunEvent(ev)
}

func (gh *globalHandle) leave() {
	if old := gh.g.Swap(nil); old != nil {
		old.remove(gh)
	}
}
