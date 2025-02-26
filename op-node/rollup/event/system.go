package event

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/log"
)

type Registry interface {
	// Register registers a named event-emitter, optionally processing events itself:
	// deriver may be nil, not all registrants have to process events.
	// A non-nil deriver may implement AttachEmitter to automatically attach the Emitter to it,
	// before the deriver itself becomes executable.
	Register(name string, deriver Deriver, opts *RegisterOpts) Emitter
	// Unregister removes a named emitter,
	// also removing it from the set of events-receiving derivers (if registered with non-nil deriver).
	Unregister(name string) (old Emitter)
}

type System interface {
	Registry
	// AddTracer registers a tracer to capture all event deriver/emitter work. It runs until RemoveTracer is called.
	// Duplicate tracers are allowed.
	AddTracer(t Tracer)
	// RemoveTracer removes a tracer. This is a no-op if the tracer was not previously added.
	// It will remove all added duplicates of the tracer.
	RemoveTracer(t Tracer)
	// Stop shuts down the System by un-registering all derivers/emitters.
	Stop()
}

type AttachEmitter interface {
	AttachEmitter(em Emitter)
}

type AnnotatedEvent struct {
	Event       Event
	EmitContext uint64 // uniquely identifies the emission of the event, useful for debugging and creating diagrams
}

// systemActor is a deriver and/or emitter, registered in System with a name.
// If deriving, the actor is added as Executable to the Executor of the System.
type systemActor struct {
	name string
	sys  *Sys

	// To manage the execution peripherals, like rate-limiting, of this deriver
	ctx    context.Context
	cancel context.CancelFunc

	deriv         Deriver
	leaveExecutor func()

	// 0 if event does not originate from Deriver-handling of another event
	currentEvent uint64
}

// Emit is called by the end-user
func (r *systemActor) Emit(ev Event) {
	if r.ctx.Err() != nil {
		return
	}
	r.sys.emit(r.name, r.currentEvent, ev)
}

// RunEvent is called by the events executor.
// While different things may execute in parallel, only one event is executed per entry at a time.
func (r *systemActor) RunEvent(ev AnnotatedEvent) {
	if r.deriv == nil {
		return
	}
	if r.ctx.Err() != nil {
		return
	}
	if r.sys.abort.Load() && !Is[CriticalErrorEvent](ev.Event) {
		// if aborting, and not the CriticalErrorEvent itself, then do not process the event
		return
	}

	prev := r.currentEvent
	start := time.Now()
	r.currentEvent = r.sys.recordDerivStart(r.name, ev, start)
	effect := r.deriv.OnEvent(ev.Event)
	elapsed := time.Since(start)
	r.sys.recordDerivEnd(r.name, ev, r.currentEvent, start, elapsed, effect)
	r.currentEvent = prev
}

// Sys is the canonical implementation of System.
type Sys struct {
	regs     map[string]*systemActor
	regsLock sync.Mutex

	log log.Logger

	executor Executor

	// used to generate a unique id for each event deriver processing call.
	derivContext atomic.Uint64
	// used to generate a unique id for each event-emission.
	emitContext atomic.Uint64

	tracers     []Tracer
	tracersLock sync.RWMutex

	// if true, no events may be processed, except CriticalError itself
	abort atomic.Bool
}

func NewSystem(log log.Logger, ex Executor) *Sys {
	return &Sys{
		regs:     make(map[string]*systemActor),
		executor: ex,
		log:      log,
	}
}

func (s *Sys) Register(name string, deriver Deriver, opts *RegisterOpts) Emitter {
	s.regsLock.Lock()
	defer s.regsLock.Unlock()

	if _, ok := s.regs[name]; ok {
		panic(fmt.Errorf("a deriver/emitter with name %q already exists", name))
	}

	ctx, cancel := context.WithCancel(context.Background())
	r := &systemActor{
		name:   name,
		deriv:  deriver,
		sys:    s,
		ctx:    ctx,
		cancel: cancel,
	}
	s.regs[name] = r
	var em Emitter = r
	if opts.Emitter.Limiting {
		limitedCallback := opts.Emitter.OnLimited
		em = NewLimiter(ctx, r, opts.Emitter.Rate, opts.Emitter.Burst, func() {
			r.sys.recordRateLimited(name, r.currentEvent)
			if limitedCallback != nil {
				limitedCallback()
			}
		})
	}

	// If it can derive, add it to the executor (and only after attaching the emitter)
	if deriver != nil {
		// If it can emit, attach an emitter to it
		if attachTo, ok := deriver.(AttachEmitter); ok {
			attachTo.AttachEmitter(em)
		}
		r.leaveExecutor = s.executor.Add(r, &opts.Executor)
	}
	return em
}

func (s *Sys) Unregister(name string) (previous Emitter) {
	s.regsLock.Lock()
	defer s.regsLock.Unlock()
	return s.unregister(name)
}

func (s *Sys) unregister(name string) (previous Emitter) {
	r, ok := s.regs[name]
	if !ok {
		return nil
	}
	r.cancel()
	// if this was registered as deriver with the executor, then leave the executor
	if r.leaveExecutor != nil {
		r.leaveExecutor()
	}
	delete(s.regs, name)
	return r
}

// Stop shuts down the system
// by unregistering all emitters/derivers,
// freeing up executor resources.
func (s *Sys) Stop() {
	s.regsLock.Lock()
	defer s.regsLock.Unlock()
	for _, r := range s.regs {
		s.unregister(r.name)
	}
}

func (s *Sys) AddTracer(t Tracer) {
	s.tracersLock.Lock()
	defer s.tracersLock.Unlock()
	s.tracers = append(s.tracers, t)
}

func (s *Sys) RemoveTracer(t Tracer) {
	s.tracersLock.Lock()
	defer s.tracersLock.Unlock()
	// We are not removing tracers often enough to optimize the deletion;
	// instead we prefer fast and simple tracer iteration during regular operation.
	s.tracers = slices.DeleteFunc(s.tracers, func(v Tracer) bool {
		return v == t
	})
}

// recordDerivStart records that the deriver by name [deriv] is processing event [ev].
// This returns a unique integer (during lifetime of Sys), usable as ID to reference processing.
func (s *Sys) recordDerivStart(name string, ev AnnotatedEvent, startTime time.Time) uint64 {
	derivContext := s.derivContext.Add(1)

	s.tracersLock.RLock()
	defer s.tracersLock.RUnlock()
	for _, t := range s.tracers {
		t.OnDeriveStart(name, ev, derivContext, startTime)
	}

	return derivContext
}

func (s *Sys) recordDerivEnd(name string, ev AnnotatedEvent, derivContext uint64, startTime time.Time, duration time.Duration, effect bool) {
	s.tracersLock.RLock()
	defer s.tracersLock.RUnlock()
	for _, t := range s.tracers {
		t.OnDeriveEnd(name, ev, derivContext, startTime, duration, effect)
	}
}

func (s *Sys) recordRateLimited(name string, derivContext uint64) {
	s.tracersLock.RLock()
	defer s.tracersLock.RUnlock()
	s.log.Warn("Event-system emitter component was rate-limited", "emitter", name)
	for _, t := range s.tracers {
		t.OnRateLimited(name, derivContext)
	}
}

func (s *Sys) recordEmit(name string, ev AnnotatedEvent, derivContext uint64, emitTime time.Time) {
	s.tracersLock.RLock()
	defer s.tracersLock.RUnlock()
	for _, t := range s.tracers {
		t.OnEmit(name, ev, derivContext, emitTime)
	}
}

// emit an event [ev] during the derivation of another event, referenced by derivContext.
// If the event was emitted not as part of deriver event execution, then the derivContext is 0.
// The name of the emitter is provided to further contextualize the event.
func (s *Sys) emit(name string, derivContext uint64, ev Event) {
	emitContext := s.emitContext.Add(1)
	annotated := AnnotatedEvent{Event: ev, EmitContext: emitContext}

	// As soon as anything emits a critical event,
	// make the system aware, before the executor event schedules it for processing.
	if Is[CriticalErrorEvent](ev) {
		s.abort.Store(true)
	}

	emitTime := time.Now()
	s.recordEmit(name, annotated, derivContext, emitTime)

	err := s.executor.Enqueue(annotated)
	if err != nil {
		s.log.Error("Failed to enqueue event", "emitter", name, "event", ev, "context", derivContext)
		return
	}
}
