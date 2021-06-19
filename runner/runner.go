package runner

import (
	"fmt"
	"sync"
	"sync/atomic"
	"unsafe"
)

type CyclicDependencyError string

func (e CyclicDependencyError) Error() string {
	return string(e)
}

type Result struct {
	Target Target
	Error  error
}

type Targets interface {
	LoadTarget(label string) (Target, error)
}

type Engine interface {
	EvaluateTargets(labels ...string) []Result
}

type Target interface {
	Evaluate(engine Engine) error
}

const (
	statusIdle = iota
	statusRunning
	statusSucceeded
	statusFailed
)

type target struct {
	m sync.Mutex
	c *sync.Cond

	label  string
	target Target

	waiting unsafe.Pointer // *[]*target

	status int
	err    error
}

func newTarget(label string) *target {
	tt := &target{label: label}
	tt.c = sync.NewCond(&tt.m)
	return tt
}

func (t *target) start(r *runner) {
	t.m.Lock()
	if t.status != statusIdle {
		t.m.Unlock()
		return
	}

	t.status = statusRunning
	t.m.Unlock()

	go t.run(r)
}

func (t *target) wait() error {
	t.m.Lock()
	defer t.m.Unlock()

	if t.status == statusRunning {
		for t.status == statusRunning {
			t.c.Wait()
		}
	}

	return t.err
}

func (t *target) run(r *runner) {
	defer func() {
		t.m.Unlock()
		t.c.Broadcast()
	}()

	// Load the target.
	tt, err := r.targetLoader.LoadTarget(t.label)
	if err != nil {
		t.m.Lock()
		t.status, t.err = statusFailed, err
		return
	}
	t.target = tt

	// Evaluate and save the target.
	status := statusSucceeded
	if err = t.target.Evaluate(&engine{root: t, runner: r}); err != nil {
		status = statusFailed
	}

	t.m.Lock()
	t.status, t.err = status, err
}

type engine struct {
	root   *target
	runner *runner
}

func (e *engine) check(dep *target) error {
	if dep == e.root {
		return CyclicDependencyError(fmt.Sprintf("cyclic dependency on %v", dep.label))
	}

	if waiting := (*[]*target)(atomic.LoadPointer(&dep.waiting)); waiting != nil {
		return e.checkDeps(*waiting)
	}
	return nil
}

func (e *engine) checkDeps(deps []*target) error {
	for _, t := range deps {
		if err := e.check(t); err != nil {
			return err
		}
	}
	return nil
}

func (e *engine) EvaluateTargets(labels ...string) []Result {
	targets := make([]*target, len(labels))
	for i, label := range labels {
		targets[i] = e.runner.getTarget(label)
		targets[i].start(e.runner)
	}

	atomic.SwapPointer(&e.root.waiting, unsafe.Pointer(&targets))
	defer atomic.SwapPointer(&e.root.waiting, nil)

	results := make([]Result, len(targets))
	if err := e.checkDeps(targets); err != nil {
		for i := range results {
			results[i].Error = err
			results[i].Target = nil
		}
		return results
	}

	for i, t := range targets {
		results[i].Error = t.wait()
		results[i].Target = t.target
	}
	return results
}

type runner struct {
	targetLoader Targets
	targetMap    sync.Map // map[string]*target
}

func (r *runner) getTarget(label string) *target {
	tv, _ := r.targetMap.LoadOrStore(label, newTarget(label))
	return tv.(*target)
}

func Run(targets Targets, label string) error {
	r := runner{targetLoader: targets}
	t := r.getTarget(label)
	t.start(&r)
	return t.wait()
}
