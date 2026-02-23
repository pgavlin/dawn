package dawn

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"

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

	return proj.typeCheckAll(ctx)
}

// typeCheckAll type-checks all BUILD.dawn files in the project.
func (proj *Project) typeCheckAll(ctx context.Context) ([]CheckResult, error) {
	pc := &projectChecker{
		proj:    proj,
		baseEnv: DawnEnv(),
		cache:   make(map[string]*checkResult),
		loading: make(map[string]bool),
	}

	walked := make(map[string]bool)
	var results []CheckResult
	err := pc.walkProject(proj.root, ".", func(path string) {
		walked[path] = true
		pc.checkModule(ctx, path, "", proj.requirements)
		if r := pc.cache[path]; r != nil && len(r.errs) > 0 {
			results = append(results, CheckResult{Path: path, Errors: r.errs})
		}
	})
	if err != nil {
		return nil, err
	}

	// Also report errors from helper modules that were loaded but not directly walked.
	// Only include modules within the project root (skip external dependencies).
	for path, r := range pc.cache {
		if walked[path] || len(r.errs) == 0 {
			continue
		}
		if rel, err := filepath.Rel(proj.root, path); err != nil || strings.HasPrefix(rel, "..") {
			continue
		}
		results = append(results, CheckResult{Path: path, Errors: r.errs})
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

// checkResult holds the cached result of checking a module.
type checkResult struct {
	types        map[string]typecheck.Type // exported name → type
	project      string                    // project path for this module (empty for local)
	requirements map[string]string         // per-module requirements from fetchModule
	errs         []typecheck.Error
}

// projectChecker coordinates type-checking across modules in a Dawn project.
type projectChecker struct {
	proj    *Project                // minimal project (config only) for fetchModule, root, ignore
	baseEnv *typecheck.Env         // shared Predeclared from DawnEnv()
	cache   map[string]*checkResult // resolved filepath → cached result
	loading map[string]bool         // filepath → currently being checked (cycle detection)
}

// walkProject recursively walks the project directory, calling fn for each
// BUILD.dawn file found. Directories matching the project's ignore patterns
// and the .dawn directory are skipped.
func (pc *projectChecker) walkProject(root, rel string, fn func(path string)) error {
	dir := filepath.Join(root, rel)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	if slices.ContainsFunc(entries, func(e os.DirEntry) bool { return e.Name() == "BUILD.dawn" }) {
		fn(filepath.Join(dir, "BUILD.dawn"))
	}

	for _, e := range entries {
		if !e.IsDir() || e.Name() == ".dawn" {
			continue
		}
		childRel := filepath.Join(rel, e.Name())
		if pc.proj.ignored(childRel) {
			continue
		}
		if err := pc.walkProject(root, childRel, fn); err != nil {
			return err
		}
	}
	return nil
}

// resolveAndCheck resolves a load label and type-checks the referenced module.
// It mirrors module.loadModule + Project.fetchModule for label resolution,
// including alias handling via per-module requirements.
func (pc *projectChecker) resolveAndCheck(ctx context.Context, rawModule, callerProject, callerPkg string, callerRequirements map[string]string) map[string]typecheck.Type {
	l, err := label.Parse(rawModule)
	if err != nil {
		return nil
	}
	if l.Project == "" {
		l.Project = callerProject
		if l.Name == "" {
			l.Name = "BUILD.dawn"
		}
	} else if l.IsAlias() {
		req, ok := callerRequirements[l.Project]
		if !ok {
			return nil
		}
		l.Project = req
	}
	l, _ = l.RelativeTo(callerPkg)

	path, moduleReqs, err := pc.proj.fetchModule(ctx, l)
	if err != nil {
		return nil
	}

	return pc.checkModule(ctx, path, l.Project, moduleReqs)
}

// pathToPackage converts an absolute filesystem path back to a label package string.
func (pc *projectChecker) pathToPackage(filePath string) string {
	dir := filepath.Dir(filePath)
	rel, err := filepath.Rel(pc.proj.root, dir)
	if err != nil {
		return "//"
	}
	if rel == "." {
		return "//"
	}
	return "//" + filepath.ToSlash(rel)
}

// envForModule creates a per-module env that shares Predeclared from
// baseEnv but has a Load closure capturing the module's package and requirements
// for relative label and alias resolution.
func (pc *projectChecker) envForModule(ctx context.Context, project, pkg string, requirements map[string]string) *typecheck.Env {
	return &typecheck.Env{
		Predeclared: pc.baseEnv.Predeclared,
		Load: func(module string) map[string]typecheck.Type {
			return pc.resolveAndCheck(ctx, module, project, pkg, requirements)
		},
	}
}

// checkModule checks a module file and returns its exported types.
// Results are cached so each module is checked at most once.
// Cycles are detected and broken gracefully (cycle → nil → Any).
func (pc *projectChecker) checkModule(ctx context.Context, path, project string, requirements map[string]string) map[string]typecheck.Type {
	if r, ok := pc.cache[path]; ok {
		return r.types
	}
	if pc.loading[path] {
		return nil
	}
	pc.loading[path] = true
	defer delete(pc.loading, path)

	src, err := os.ReadFile(path)
	if err != nil {
		pc.cache[path] = &checkResult{project: project, requirements: requirements, errs: []typecheck.Error{{Msg: err.Error()}}}
		return nil
	}

	f, err := syntax.Parse(path, src, 0)
	if err != nil {
		var errs []typecheck.Error
		var synErr syntax.Error
		if errors.As(err, &synErr) {
			errs = []typecheck.Error{{Pos: synErr.Pos, Msg: synErr.Msg}}
		} else {
			errs = []typecheck.Error{{Msg: err.Error()}}
		}
		pc.cache[path] = &checkResult{project: project, requirements: requirements, errs: errs}
		return nil
	}

	pkg := pc.pathToPackage(path)
	env := pc.envForModule(ctx, project, pkg, requirements)

	if err := resolve.File(f, isPredeclared(env), isUniversal); err != nil {
		var errs []typecheck.Error
		var resolveErrs resolve.ErrorList
		if errors.As(err, &resolveErrs) {
			errs = make([]typecheck.Error, len(resolveErrs))
			for i, e := range resolveErrs {
				errs[i] = typecheck.Error{Pos: e.Pos, Msg: e.Msg}
			}
		} else {
			errs = []typecheck.Error{{Msg: err.Error()}}
		}
		pc.cache[path] = &checkResult{project: project, requirements: requirements, errs: errs}
		return nil
	}

	info := &typecheck.Info{Defs: make(map[*syntax.Ident]*typecheck.Binding)}
	errs := typecheck.Check(f, env, info)
	exports := extractExports(f, info)
	pc.cache[path] = &checkResult{types: exports, project: project, requirements: requirements, errs: errs}
	return exports
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
