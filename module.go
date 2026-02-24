package dawn

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/pgavlin/dawn/label"
	"github.com/pgavlin/dawn/util"
	"github.com/pgavlin/starlark-go/resolve"
	"github.com/pgavlin/starlark-go/starlark"
	"github.com/pgavlin/starlark-go/syntax"
	"github.com/pgavlin/starlark-go/typecheck"
)

// A module is the runtime representation of a Starlark module.
type module struct {
	m    sync.Mutex
	cond *sync.Cond

	dependencies []string

	label        *label.Label
	path         string
	projectPath  string
	requirements map[string]string

	// Type-check state (populated during Open)
	file        *syntax.File    // retained parsed AST (with comments)
	checkInfo   *typecheck.Info // retained type info (Defs, Uses, Types)
	checkErrs   []typecheck.Error
	exportTypes map[string]typecheck.Type

	// Eval state — true once a goroutine has claimed this module for evaluation
	evaluating bool

	loaded bool
	data   starlark.StringDict
	err    error

	out *lineWriter
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

// wait waits for the receiver to finish loading.
func (m *module) wait() (starlark.StringDict, error) {
	m.m.Lock()
	defer m.m.Unlock()

	for !m.loaded {
		m.cond.Wait()
	}

	return m.data, m.err
}

// env returns a thread and builtins appropriate for running this module's code.
func (m *module) env(proj *Project) (*starlark.Thread, starlark.StringDict, error) {
	if m.path == "" {
		path, moduleReqs, err := proj.fetchModule(context.TODO(), m.label)
		if err != nil {
			return nil, nil, err
		}
		m.path, m.requirements = path, moduleReqs
	}

	t := starlark.Thread{
		Name: m.label.String(),
		Print: func(t *starlark.Thread, msg string) {
			proj.events.Print(m.label, msg)
		},
		Load: func(t *starlark.Thread, rawLabel string) (starlark.StringDict, error) {
			return m.loadModule(util.GetContext(t), proj, rawLabel)
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
	builtins["fail"] = proj.newBuiltin_fail()

	builtins["package"] = starlark.String(m.label.Package)

	return &t, builtins, nil
}

func (m *module) resolveLabel(rawLabel string) (*label.Label, error) {
	l, err := label.Parse(rawLabel)
	if err != nil {
		return nil, err
	}
	if l.Project == "" {
		l.Project = m.label.Project
		if l.Name == "" {
			l.Name = "BUILD.dawn"
		}
	} else if l.IsAlias() {
		req, ok := m.requirements[l.Project]
		if !ok {
			return nil, fmt.Errorf("no project with alias %q", l.Project)
		}
		l.Project = req
	}
	l, _ = l.RelativeTo(m.label.Package)
	l.Kind = "module"
	return l, nil
}

func (m *module) typeCheck(ctx context.Context, proj *Project, baseEnv *typecheck.Env, opening map[string]bool) {
	src, err := os.ReadFile(m.path)
	if err != nil {
		m.checkErrs = []typecheck.Error{{Msg: err.Error()}}
		return
	}

	f, err := syntax.Parse(m.path, src, syntax.RetainComments)
	if err != nil {
		var synErr syntax.Error
		if errors.As(err, &synErr) {
			m.checkErrs = []typecheck.Error{{Pos: synErr.Pos, Msg: synErr.Msg}}
		} else {
			m.checkErrs = []typecheck.Error{{Msg: err.Error()}}
		}
		return
	}
	m.file = f

	env := &typecheck.Env{
		Predeclared: baseEnv.Predeclared,
		Load: func(rawModule string) map[string]typecheck.Type {
			l, err := m.resolveLabel(rawModule)
			if err != nil {
				return nil
			}
			dep := proj.openModule(ctx, l, opening)
			if dep == nil {
				return nil
			}
			return dep.exportTypes
		},
	}

	if err := resolve.File(f, IsPredeclared(env), IsUniversal); err != nil {
		var resolveErrs resolve.ErrorList
		if errors.As(err, &resolveErrs) {
			m.checkErrs = make([]typecheck.Error, len(resolveErrs))
			for i, e := range resolveErrs {
				m.checkErrs[i] = typecheck.Error{Pos: e.Pos, Msg: e.Msg}
			}
		} else {
			m.checkErrs = []typecheck.Error{{Msg: err.Error()}}
		}
		return
	}

	info := &typecheck.Info{
		Defs:  make(map[*syntax.Ident]*typecheck.Binding),
		Uses:  make(map[*syntax.Ident]*typecheck.UseBinding),
		Types: make(map[syntax.Expr]typecheck.TypeAndValue),
	}
	m.checkErrs = typecheck.Check(f, env, info)
	m.checkInfo = info
	m.exportTypes = extractExports(f, info)
}

func (m *module) loadModule(ctx context.Context, proj *Project, rawLabel string) (starlark.StringDict, error) {
	l, err := m.resolveLabel(rawLabel)
	if err != nil {
		return nil, err
	}

	m.dependencies = append(m.dependencies, l.String())
	return proj.loadModule(ctx, l)
}

// load executes the module's code.
func (m *module) load(ctx context.Context, proj *Project) (starlark.StringDict, error) {
	proj.events.ModuleLoading(m.label)

	t, builtins, err := m.env(proj)
	if err != nil {
		proj.events.ModuleLoadFailed(m.label, err)
		return nil, err
	}
	done := util.SetContext(ctx, t)
	defer done()

	v, err := m.done(starlark.ExecFile(t, m.path, nil, builtins))
	if err != nil {
		proj.events.ModuleLoadFailed(m.label, err)
		return nil, err
	}

	proj.events.ModuleLoaded(m.label)
	return v, nil
}
