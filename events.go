package dawn

import (
	"github.com/pgavlin/dawn/label"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// Events allows callers to handle project load and build events.
type Events interface {
	// Print logs a line of output associated with a module or target.
	Print(label *label.Label, line string)

	// ModuleLoading is called when the given module begins loading.
	ModuleLoading(label *label.Label)
	// ModuleLoaded is called when the given module finishes loading successfully.
	ModuleLoaded(label *label.Label)
	// ModuleLoadFailed is called when the given module fails to load.
	ModuleLoadFailed(label *label.Label, err error)
	// LoadDone is called when a project finishes loading.
	LoadDone(err error)

	// TargetUpToDate is called when a target is found to be up-to-date.
	TargetUpToDate(label *label.Label)
	// TargetEvaluating is called when a target begins executing.
	TargetEvaluating(label *label.Label, reason string)
	// TargetFailed is called when a target fails.
	TargetFailed(label *label.Label, err error)
	// TargetSucceeded is called when a target succeeds.
	TargetSucceeded(label *label.Label, changed bool)
	// RunDone is called when a run finishes.
	RunDone(err error)
}

type discardEventsT int

// DiscardEvents is an implementation of Events that discards all events.
var DiscardEvents = discardEventsT(0)

func (discardEventsT) Print(label *label.Label, line string)              {}
func (discardEventsT) ModuleLoading(label *label.Label)                   {}
func (discardEventsT) ModuleLoaded(label *label.Label)                    {}
func (discardEventsT) ModuleLoadFailed(label *label.Label, err error)     {}
func (discardEventsT) LoadDone(err error)                                 {}
func (discardEventsT) TargetUpToDate(label *label.Label)                  {}
func (discardEventsT) TargetEvaluating(label *label.Label, reason string) {}
func (discardEventsT) TargetFailed(label *label.Label, err error)         {}
func (discardEventsT) TargetSucceeded(label *label.Label, changed bool)   {}
func (discardEventsT) RunDone(err error)                                  {}

type runEvents struct {
	c        chan starlark.Value
	callback starlark.Callable
	done     chan bool
}

func (e *runEvents) process(thread *starlark.Thread) {
	for event := range e.c {
		if e.callback != nil {
			starlark.Call(thread, e.callback, starlark.Tuple{event}, nil)
		}
	}
	close(e.done)
}

func (e *runEvents) Close() {
	close(e.c)
	<-e.done
}

func (*runEvents) ModuleLoading(label *label.Label)               {}
func (*runEvents) ModuleLoaded(label *label.Label)                {}
func (*runEvents) ModuleLoadFailed(label *label.Label, err error) {}
func (*runEvents) LoadDone(err error)                             {}

func (e *runEvents) Print(label *label.Label, line string) {
	e.c <- starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"kind":  starlark.String("Print"),
		"label": starlark.String(label.String()),
		"line":  starlark.String(line),
	})
}

func (e *runEvents) TargetUpToDate(label *label.Label) {
	e.c <- starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"kind":  starlark.String("TargetUpToDate"),
		"label": starlark.String(label.String()),
	})
}

func (e *runEvents) TargetEvaluating(label *label.Label, reason string) {
	e.c <- starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"kind":   starlark.String("TargetEvaluating"),
		"label":  starlark.String(label.String()),
		"reason": starlark.String(reason),
	})

}

func (e *runEvents) TargetFailed(label *label.Label, err error) {
	e.c <- starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"kind":  starlark.String("TargetUpToDate"),
		"label": starlark.String(label.String()),
		"err":   starlark.String(err.Error()),
	})

}

func (e *runEvents) TargetSucceeded(label *label.Label, changed bool) {
	e.c <- starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"kind":    starlark.String("TargetSucceeded"),
		"label":   starlark.String(label.String()),
		"changed": starlark.Bool(changed),
	})

}

func (e *runEvents) RunDone(err error) {
	msg := starlark.Value(starlark.None)
	if err != nil {
		msg = starlark.String(err.Error())
	}

	e.c <- starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"kind": starlark.String("RunDone"),
		"err":  msg,
	})
}
