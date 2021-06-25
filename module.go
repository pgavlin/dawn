package dawn

import (
	"fmt"
	"path/filepath"
	"sync"

	"github.com/pgavlin/dawn/label"
	"github.com/pgavlin/dawn/util"
	"go.starlark.net/starlark"
)

// A module is the runtime representation of a Starlark module.
type module struct {
	m    sync.Mutex
	cond *sync.Cond

	loading *module

	dependencies []string

	label *label.Label
	path  string

	loaded bool
	data   starlark.StringDict
	err    error

	out *lineWriter
}

// getLoading returns the module (if any) being loaded by the receiver.
func (m *module) getLoading() *module {
	m.m.Lock()
	defer m.m.Unlock()
	return m.loading
}

// setLoading marks the receiver as waiting on the given module.
func (m *module) setLoading(other *module) {
	m.m.Lock()
	m.loading = other
	m.m.Unlock()
}

// done marks the receiver as done.
func (m *module) done(data starlark.StringDict, err error) (starlark.StringDict, error) {
	m.data, m.err = data, err

	m.m.Lock()
	m.loaded = true
	m.m.Unlock()
	m.cond.Broadcast()

	return data, err
}

// wait waits for the receiver to finish loading. It returns an error if the module fails
// to load or if the wait would result in a cyclic dependency.
func (m *module) wait(waiter *module) (starlark.StringDict, error) {
	m.m.Lock()
	defer m.m.Unlock()

	if waiter != nil {
		loading := m.loading
		for loading != nil {
			if loading == waiter {
				return nil, fmt.Errorf("cyclic dependency on %v", m.label)
			}
			loading = m.getLoading()
		}
	}

	for !m.loaded {
		m.cond.Wait()
	}

	return m.data, m.err
}

// env returns a thread and builtins appropriate for running this module's code.
func (m *module) env(proj *Project) (*starlark.Thread, starlark.StringDict, error) {
	path, err := proj.fetchModule(m.label)
	if err != nil {
		return nil, nil, err
	}
	m.path = path

	t := starlark.Thread{
		Name: m.label.String(),
		Print: func(t *starlark.Thread, msg string) {
			proj.events.Print(m.label, msg)
		},
		Load: func(t *starlark.Thread, rawLabel string) (starlark.StringDict, error) {
			label, err := label.Parse(rawLabel)
			if err != nil {
				return nil, err
			}
			label, _ = label.RelativeTo(m.label.Package)
			label.Kind = "module"

			m.dependencies = append(m.dependencies, label.String())
			return proj.loadModule(m, label)
		},
	}

	util.SetStdio(&t, m.out, m.out)
	util.Chdir(&t, filepath.Dir(m.path))

	t.SetLocal("root", proj.root)
	t.SetLocal("module", m)

	// make a module-local copy of the builtins so we can add a few of our own.
	builtins := starlark.StringDict{}
	for k, v := range proj.builtins {
		builtins[k] = v
	}

	builtins["host"] = builtin_host

	builtins["Cache"] = builtin_cache
	builtins["path"] = proj.newBuiltin_path()
	builtins["label"] = proj.newBuiltin_label()
	builtins["contains"] = proj.newBuiltin_contains()
	builtins["parse_flag"] = proj.newBuiltin_parse_flag()
	builtins["target"] = proj.newBuiltin_target()
	builtins["glob"] = proj.newBuiltin_glob()

	builtins["package"] = starlark.String(m.label.Package)

	return &t, builtins, nil
}

// load executes the module's code.
func (m *module) load(proj *Project) (starlark.StringDict, error) {
	proj.events.ModuleLoading(m.label)

	t, builtins, err := m.env(proj)
	if err != nil {
		proj.events.ModuleLoadFailed(m.label, err)
		return nil, err
	}

	v, err := m.done(starlark.ExecFile(t, m.path, nil, builtins))
	if err != nil {
		proj.events.ModuleLoadFailed(m.label, err)
		return nil, err
	}

	proj.events.ModuleLoaded(m.label)
	return v, nil
}
