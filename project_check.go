package dawn

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/pgavlin/dawn/label"
	"github.com/pgavlin/starlark-go/resolve"
	"github.com/pgavlin/starlark-go/syntax"
	"github.com/pgavlin/starlark-go/typecheck"
)

// CheckOptions configures project type-checking.
type CheckOptions struct {
	Events Events
}

// CheckResult holds type-checking errors for a single file.
type CheckResult struct {
	Path   string
	Errors []typecheck.Error
}

// Check type-checks all BUILD.dawn files in the project at root.
// Module loads are resolved using the same logic as Project.Load,
// including MVS dependency resolution and project alias handling.
func Check(ctx context.Context, root string, options *CheckOptions) ([]CheckResult, error) {
	var events Events
	if options != nil {
		events = options.Events
	}

	proj, err := newProjectForCheck(root, events)
	if err != nil {
		return nil, err
	}

	return proj.openModules(ctx)
}

// openModule creates or retrieves a module in proj.modules, populates its
// path and requirements via fetchModule, then type-checks it. The opening
// map provides cycle detection (single-threaded during Open).
func (proj *Project) openModule(ctx context.Context, l *label.Label, opening map[string]bool) *module {
	key := l.String()
	if m, ok := proj.modules[key]; ok {
		return m
	}
	if opening[key] {
		return nil // Cycle — break gracefully
	}
	opening[key] = true
	defer delete(opening, key)

	m := &module{label: l, out: newLineWriter(l, proj.events)}
	m.cond = sync.NewCond(&m.m)

	path, reqs, err := proj.fetchModule(ctx, l)
	if err != nil {
		m.checkErrs = []typecheck.Error{{Msg: err.Error()}}
		proj.modules[key] = m
		return m
	}
	m.path, m.requirements = path, reqs
	proj.modules[key] = m

	proj.events.ModuleOpening(l)
	m.typeCheck(ctx, proj, proj.baseEnv, opening)
	if len(m.checkErrs) > 0 {
		proj.events.ModuleOpenFailed(l, fmt.Errorf("%d type-check error(s)", len(m.checkErrs)))
	} else {
		proj.events.ModuleOpened(l)
	}
	return m
}

// openModules walks the project tree, creates module structs, and type-checks them.
// This is the sole source of module discovery — Load() does not walk the filesystem.
func (proj *Project) openModules(ctx context.Context) ([]CheckResult, error) {
	proj.baseEnv = DawnEnv()
	opening := make(map[string]bool)

	err := proj.walkPackages(proj.root, ".", func(pkg string) {
		l := &label.Label{Kind: "module", Package: pkg, Name: "BUILD.dawn"}
		proj.openModule(ctx, l, opening)
	})
	if err != nil {
		return nil, err
	}

	// Collect check results from all opened modules (including transitive deps)
	var results []CheckResult
	for _, m := range proj.modules {
		if len(m.checkErrs) == 0 || m.path == "" {
			continue
		}
		// Only report errors for modules within the project root
		if rel, err := filepath.Rel(proj.root, m.path); err == nil && !strings.HasPrefix(rel, "..") {
			results = append(results, CheckResult{Path: m.path, Errors: m.checkErrs})
		}
	}
	return results, nil
}

// CheckFile parses, resolves, and type-checks a single Starlark file.
func CheckFile(filename string, src []byte, env *typecheck.Env) []typecheck.Error {
	f, err := syntax.Parse(filename, src, 0)
	if err != nil {
		var synErr syntax.Error
		if errors.As(err, &synErr) {
			return []typecheck.Error{{Pos: synErr.Pos, Msg: synErr.Msg}}
		}
		return []typecheck.Error{{Msg: err.Error()}}
	}

	if err := resolve.File(f, isPredeclared(env), isUniversal); err != nil {
		var resolveErrs resolve.ErrorList
		if errors.As(err, &resolveErrs) {
			errs := make([]typecheck.Error, len(resolveErrs))
			for i, e := range resolveErrs {
				errs[i] = typecheck.Error{Pos: e.Pos, Msg: e.Msg}
			}
			return errs
		}
		return []typecheck.Error{{Msg: err.Error()}}
	}

	return typecheck.Check(f, env, nil)
}

// extractExports walks the top-level statements of a checked file and collects
// exported names and their types from info.Defs.
func extractExports(f *syntax.File, info *typecheck.Info) map[string]typecheck.Type {
	exports := make(map[string]typecheck.Type)
	for _, stmt := range f.Stmts {
		switch s := stmt.(type) {
		case *syntax.DefStmt:
			if b, ok := info.Defs[s.Name]; ok {
				exports[s.Name.Name] = b.Type
			}
		case *syntax.AssignStmt:
			if s.Op == syntax.EQ {
				if id, ok := s.LHS.(*syntax.Ident); ok {
					if b, ok := info.Defs[id]; ok {
						exports[id.Name] = b.Type
					}
				}
			}
		case *syntax.LoadStmt:
			for _, to := range s.To {
				if b, ok := info.Defs[to]; ok {
					exports[to.Name] = b.Type
				}
			}
		}
	}
	return exports
}
