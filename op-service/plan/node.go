package plan

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/ethereum-optimism/optimism/op-service/locks"
)

var ErrNotReady = errors.New("not ready for reads (forgot to register a dependency or eval func/val?)")

// reasonGen is used to generate unique values, so we can de-duplicate invalidation work
var reasonGen = func() *atomic.Uint64 {
	v := new(atomic.Uint64)
	v.Add(1)
	return v
}()

type downstreamDep interface {
	depInvalidate(reason uint64)
}

var _ downstreamDep = (*Lazy[struct{}])(nil)

type upstreamDep interface {
	depEval(ctx context.Context) error
	rlock()
	runlock()
	register(d downstreamDep)
	unregister(d downstreamDep)
	Err() error
	String() string
}

var _ upstreamDep = (*Lazy[struct{}])(nil)

// Lazy is a lazily-evaluated value, that can be invalidated and re-evaluated.
// Lazy can depend on other values, and will then be invalidated if those values get invalidated.
// This enables construction of directed graphs of dependent values,
// to declare computation rather than using imperative execution.
type Lazy[V any] struct {
	mu sync.RWMutex

	val       V
	err       error
	hasVOrErr bool // when eval has

	// reason is a temporary value, checked/changed only during invalidation.
	// This de-duplicates invalidation work.
	reason uint64

	// fn is evaluated to determine val and err
	fn func(ctx context.Context) (V, error)

	// upstream is the list of values.
	// This is a list, so order of dependencies evaluation is deterministic.
	// (wrapped with a lock so we can String while busy)
	upstream locks.RWValue[[]upstreamDep]

	// downstream is the set of values to invalidate whenever this value itself is invalidated.
	// (wrapped with a lock so we dependencies can be registered while busy)
	downstream locks.RWValue[map[downstreamDep]struct{}]
}

// Eval retrieves the value/error,
// and if necessary runs the fn to determine the values.
// Upstream dependencies are evaluated first if necessary.
// After evaluating upstream dependencies, they are all read-locked and checked for validity.
// If upstream dependencies are not valid, the fn does not run.
// Unchecked Value() access of upstream dependencies is thus safe during fn execution.
func (p *Lazy[V]) Eval(ctx context.Context) (V, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.eval(ctx)
}

func (p *Lazy[V]) eval(ctx context.Context) (V, error) {
	var empty V
	if p.hasVOrErr {
		return p.val, p.err
	}
	if p.fn == nil {
		return empty, ErrNotReady
	}

	// error or not, the change will invalidate downstream dependencies
	p.invalidate()

	p.upstream.RLock()
	defer p.upstream.RUnlock()
	// evals write-lock, so we have to complete them first
	for i, up := range p.upstream.Value {
		err := up.depEval(ctx)
		if err != nil {
			p.err = fmt.Errorf("upstream dep %d (%T) failed: %w", i, up, err)
			p.val = empty
			p.hasVOrErr = true
			return empty, p.err
		}
	}

	// now read-lock everything upstream, so the fn can have access freely
	for i, up := range p.upstream.Value {
		up.rlock()
		runlock := up.runlock
		//goland:noinspection GoDeferInLoop
		defer runlock() // intentional, we have to unlock on any exit condition
		// Check that the value is there. There was a gap between the eval and the read-lock.
		if err := up.Err(); err != nil {
			return empty, fmt.Errorf("upstream dep %d (%T) has error: %w", i, up, err)
		}
	}
	// We didn't have a value already,
	// so no need to invalidate here as things change
	v, err := p.fn(ctx)
	p.val = v
	p.err = err
	p.hasVOrErr = true
	return v, err
}

// depEval is used to fit a generic interface, used to call eval on upstream deps.
func (p *Lazy[V]) depEval(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	_, err := p.eval(ctx)
	return err
}

// rlock is used to make this readable during eval of a downstream dep
func (p *Lazy[V]) rlock() {
	p.mu.RLock()
}

// runlock is used to stop rlock
func (p *Lazy[V]) runlock() {
	p.mu.RUnlock()
}

// register is called by new downstream deps,
// so the downstream dep can be removed.
func (p *Lazy[V]) register(d downstreamDep) {
	p.downstream.Lock()
	defer p.downstream.Unlock()
	if p.downstream.Value == nil {
		p.downstream.Value = make(map[downstreamDep]struct{})
	}
	p.downstream.Value[d] = struct{}{}
}

// unregister is called by existing downstream deps, when they no longer need to be invalidated.
func (p *Lazy[V]) unregister(d downstreamDep) {
	p.downstream.Lock()
	defer p.downstream.Unlock()
	delete(p.downstream.Value, d)
}

// DependOn adds an upstream dependency, and invalidates any existing value/error
func (p *Lazy[V]) DependOn(dep ...upstreamDep) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.upstream.Lock()
	defer p.upstream.Unlock()
	// register with all the upstream deps, so we get invalidated when upstream changes
	for _, d := range dep {
		d.register(p)
	}
	p.upstream.Value = append(p.upstream.Value, dep...)
	p.invalidate()
}

// Set invalidates any downstream deps, and sets the value.
func (p *Lazy[V]) Set(v V) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.invalidate() // a changing value invalidates all downstream deps
	p.val = v
	p.err = nil
	p.hasVOrErr = true
}

// SetError invalidates any downstream deps, and empties the current value, and sets the error.
func (p *Lazy[V]) SetError(err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.invalidate() // a changing error invalidates all downstream deps
	var empty V
	p.val = empty
	p.err = err
	p.hasVOrErr = true
}

func (p *Lazy[V]) depInvalidate(reason uint64) {
	// this check prevents the value from being invalidated multiple times for the same reason
	if p.reason == reason {
		return
	}
	var empty V
	p.val = empty
	p.err = nil
	p.hasVOrErr = false
	p.downstream.RLock()
	defer p.downstream.RUnlock()
	for d := range p.downstream.Value {
		d.depInvalidate(reason)
	}
	p.reason = reason
}

// invalidate invalidates the current value, and all downstream dependencies
func (p *Lazy[V]) invalidate() {
	p.depInvalidate(reasonGen.Add(1))
}

// Fn sets what makes this Lazy lazily compute the value.
// Changing this also invalidates any downstream dependencies.
func (p *Lazy[V]) Fn(fn func(ctx context.Context) (V, error)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.invalidate()
	p.fn = fn
}

// Value retrieves the evaluated value, assuming it is set (evaluated, and not an error).
func (p *Lazy[V]) Value() V {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if !p.hasVOrErr {
		panic(ErrNotReady)
	}
	return p.val
}

// Get retrieves the value and error.
// If not ready (no value/error set) this returns a not-ready error.
func (p *Lazy[V]) Get() (v V, err error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if !p.hasVOrErr {
		err = ErrNotReady
		return
	}
	return p.val, p.err
}

// Err retrieves the evaluated error, assuming it is set (evaluated, and not an error).
// Err may return nil if the node has been set/evaluated, but did not return any error.
// If not ready (no value/error set) this returns a not-ready error.
func (p *Lazy[V]) Err() error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if !p.hasVOrErr {
		return ErrNotReady
	}
	return p.err
}

// Close cleans up,
func (p *Lazy[V]) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	// remove from upstream deps, and clear them
	p.upstream.Lock()
	defer p.upstream.Unlock()
	for _, d := range p.upstream.Value {
		d.unregister(p)
	}
	p.upstream.Value = nil

	// invalidate downstream deps, and clear them
	p.fn = nil
	p.invalidate()
	p.downstream.Set(nil)
}

func (p *Lazy[V]) String() string {
	p.upstream.RLock()
	defer p.upstream.RUnlock()
	out := fmt.Sprintf("%T(", p)
	for i, up := range p.upstream.Value {
		if i > 0 {
			out += ", "
		}
		out += fmt.Sprintf("%T", up)
	}
	out += ")"
	return out
}
