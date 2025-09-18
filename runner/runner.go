package runner

import (
	"fmt"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
)

type CyclicDependencyError struct {
	On   string
	Path []string
}

func (e *CyclicDependencyError) Error() string {
	return fmt.Sprintf("cyclic dependency on %v", e.On)
}

func (e *CyclicDependencyError) Trace() string {
	var out strings.Builder
	fmt.Fprintf(&out, "%v:\n", e.Error())
	for _, label := range slices.Backward(e.Path) {
		fmt.Fprintf(&out, "  %v\n", label)
	}
	return out.String()
}

type Result struct {
	Target Target
	Error  error
}

type Targets interface {
	LoadTarget(label string) (Target, error)
}

type Engine interface {
	Path() []string
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

	waiting atomic.Pointer[[]*target]

	status int
	err    error
}

func newTarget(label string) *target {
	tt := &target{label: label}
	tt.c = sync.NewCond(&tt.m)
	return tt
}

func (t *target) start(path []string, r *runner) {
	t.m.Lock()
	if t.status != statusIdle {
		t.m.Unlock()
		return
	}

	t.status = statusRunning
	t.m.Unlock()

	go t.run(path, r)
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

func (t *target) run(path []string, r *runner) {
	unlock := func() {
		t.m.Unlock()
		t.c.Broadcast()
	}

	r.gate.enter()
	defer r.gate.exit()

	// Load the target.
	tt, err := r.targetLoader.LoadTarget(t.label)
	if err != nil {
		t.m.Lock()
		defer unlock()

		t.status, t.err = statusFailed, err
		return
	}
	t.target = tt

	// Evaluate and save the target.
	status := statusSucceeded
	if err = t.target.Evaluate(&engine{path: path, root: t, runner: r}); err != nil {
		status = statusFailed
	}

	t.m.Lock()
	defer unlock()
	t.status, t.err = status, err
}

type engine struct {
	path   []string
	root   *target
	runner *runner
}

func (e *engine) check(path []string, dep *target) error {
	if dep == e.root {
		return &CyclicDependencyError{On: dep.label, Path: path}
	}

	if waiting := dep.waiting.Load(); waiting != nil {
		return e.checkDeps(append(path, dep.label), *waiting)
	}
	return nil
}

func (e *engine) checkDeps(path []string, deps []*target) error {
	for _, t := range deps {
		if err := e.check(path, t); err != nil {
			return err
		}
	}
	return nil
}

func (e *engine) Path() []string {
	return e.path
}

func (e *engine) EvaluateTargets(labels ...string) []Result {
	e.runner.gate.exit()
	defer e.runner.gate.enter()

	path := append(e.path, e.root.label)

	targets := make([]*target, len(labels))
	for i, label := range labels {
		targets[i] = e.runner.getTarget(label)
		targets[i].start(path, e.runner)
	}

	e.root.waiting.Swap(&targets)
	defer e.root.waiting.Swap(nil)

	results := make([]Result, len(targets))
	if err := e.checkDeps([]string{e.root.label}, targets); err != nil {
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

type gate struct {
	m        sync.Mutex
	cond     *sync.Cond
	capacity int
}

func newGate(capacity int) *gate {
	g := &gate{capacity: capacity}
	g.cond = sync.NewCond(&g.m)
	return g
}

func (g *gate) enter() {
	g.m.Lock()
	defer g.m.Unlock()

	for g.capacity == 0 {
		g.cond.Wait()
	}
	g.capacity--
}

func (g *gate) exit() {
	g.m.Lock()
	defer g.m.Unlock()

	g.capacity++
	g.cond.Signal()
}

type runner struct {
	targetLoader Targets
	targetMap    sync.Map // map[string]*target
	gate         *gate
}

func (r *runner) getTarget(label string) *target {
	tv, _ := r.targetMap.LoadOrStore(label, newTarget(label))
	return tv.(*target)
}

func Run(targets Targets, label string) error {
	r := runner{targetLoader: targets, gate: newGate(runtime.NumCPU())}
	t := r.getTarget(label)
	t.start(nil, &r)
	return t.wait()
}
