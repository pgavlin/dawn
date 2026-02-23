package check

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"github.com/pgavlin/dawn/internal/project"
	"github.com/pgavlin/glob"
	"github.com/pgavlin/starlark-go/resolve"
	"github.com/pgavlin/starlark-go/syntax"
	"github.com/pgavlin/starlark-go/typecheck"
)

// DawnEnv returns a typecheck.Env that includes both the standard Starlark
// built-ins and all Dawn-specific predeclared names and type descriptors.
func DawnEnv() *typecheck.Env {
	std := typecheck.StandardEnv()

	env := &typecheck.Env{
		Names:           make(map[string]typecheck.Type),
		TypeDescriptors: make(map[string]*typecheck.TypeDescriptor),
	}

	// Copy standard env names.
	for k, v := range std.Names {
		env.Names[k] = v
	}

	// Also register struct and module as universals.
	env.Names["struct"] = &typecheck.Callable{
		Name:       "struct",
		Params:     []typecheck.Param{{Name: "kwargs", Type: typecheck.Any, StarStar: true}},
		ReturnType: typecheck.Any,
	}
	env.Names["module"] = &typecheck.Callable{
		Name:       "module",
		Params:     []typecheck.Param{{Name: "name", Type: typecheck.String}, {Name: "kwargs", Type: typecheck.Any, StarStar: true}},
		ReturnType: typecheck.Any,
	}

	// host object
	env.Names["host"] = &typecheck.Object{
		Name: "host",
		Attrs: map[string]typecheck.Type{
			"arch": typecheck.String,
			"os":   typecheck.String,
		},
	}

	// Cache constructor
	env.Names["Cache"] = &typecheck.Callable{
		Name:       "Cache",
		ReturnType: &typecheck.Named{Name: "Cache"},
	}

	// path(label: str) -> str
	env.Names["path"] = &typecheck.Callable{
		Name:       "path",
		Params:     []typecheck.Param{{Name: "label", Type: typecheck.String}},
		ReturnType: typecheck.String,
	}

	// label(path: str) -> str
	env.Names["label"] = &typecheck.Callable{
		Name:       "label",
		Params:     []typecheck.Param{{Name: "path", Type: typecheck.String}},
		ReturnType: typecheck.String,
	}

	// contains(path: str) -> tuple[str | None, bool]
	env.Names["contains"] = &typecheck.Callable{
		Name:   "contains",
		Params: []typecheck.Param{{Name: "path", Type: typecheck.String}},
		ReturnType: &typecheck.Tuple{Elems: []typecheck.Type{
			&typecheck.Union{Types: []typecheck.Type{typecheck.String, typecheck.None}},
			typecheck.Bool,
		}},
	}

	// parse_flag(name, default=None, type=None, choices=None, required=None, help=None) -> any
	env.Names["parse_flag"] = &typecheck.Callable{
		Name: "parse_flag",
		Params: []typecheck.Param{
			{Name: "name", Type: typecheck.String},
			{Name: "default", Type: typecheck.Any, Optional: true},
			{Name: "type", Type: typecheck.Any, Optional: true},
			{Name: "choices", Type: &typecheck.List{Elem: typecheck.Any}, Optional: true},
			{Name: "required", Type: typecheck.Bool, Optional: true},
			{Name: "help", Type: typecheck.String, Optional: true},
		},
		ReturnType: typecheck.Any,
	}

	// target(...) -> Target
	env.Names["target"] = &typecheck.Callable{
		Name: "target",
		Params: []typecheck.Param{
			{Name: "name", Type: typecheck.String, Optional: true},
			{Name: "deps", Type: &typecheck.List{Elem: typecheck.Any}, Optional: true},
			{Name: "sources", Type: &typecheck.List{Elem: typecheck.String}, Optional: true},
			{Name: "generates", Type: &typecheck.List{Elem: typecheck.String}, Optional: true},
			{Name: "function", Type: typecheck.Any, Optional: true},
			{Name: "default", Type: typecheck.Bool, Optional: true},
			{Name: "always", Type: typecheck.Bool, Optional: true},
			{Name: "docs", Type: typecheck.String, Optional: true},
		},
		ReturnType: &typecheck.Named{Name: "Target"},
	}

	// glob(include, exclude=None, dirs=None) -> list[str]
	// include/exclude accept both str and list[str] (via util.StringList.Unpack)
	stringOrStringList := &typecheck.Union{Types: []typecheck.Type{
		typecheck.String,
		&typecheck.List{Elem: typecheck.String},
	}}
	env.Names["glob"] = &typecheck.Callable{
		Name: "glob",
		Params: []typecheck.Param{
			{Name: "include", Type: stringOrStringList},
			{Name: "exclude", Type: stringOrStringList, Optional: true},
			{Name: "dirs", Type: typecheck.Bool, Optional: true},
		},
		ReturnType: &typecheck.List{Elem: typecheck.String},
	}

	// fail(message: str) -> None
	env.Names["fail"] = &typecheck.Callable{
		Name:       "fail",
		Params:     []typecheck.Param{{Name: "message", Type: typecheck.String}},
		ReturnType: typecheck.None,
	}

	// run(label_or_target, always=None, dry_run=None, callback=None) -> None
	env.Names["run"] = &typecheck.Callable{
		Name: "run",
		Params: []typecheck.Param{
			{Name: "label_or_target", Type: typecheck.Any},
			{Name: "always", Type: typecheck.Bool, Optional: true},
			{Name: "dry_run", Type: typecheck.Bool, Optional: true},
			{Name: "callback", Type: typecheck.Any, Optional: true},
		},
		ReturnType: typecheck.None,
	}

	// get_target(label: str) -> Target
	env.Names["get_target"] = &typecheck.Callable{
		Name:       "get_target",
		Params:     []typecheck.Param{{Name: "label", Type: typecheck.String}},
		ReturnType: &typecheck.Named{Name: "Target"},
	}

	// flags() -> list[Flag]
	env.Names["flags"] = &typecheck.Callable{
		Name:       "flags",
		ReturnType: &typecheck.List{Elem: &typecheck.Named{Name: "Flag"}},
	}

	// targets() -> list[Target]
	env.Names["targets"] = &typecheck.Callable{
		Name:       "targets",
		ReturnType: &typecheck.List{Elem: &typecheck.Named{Name: "Target"}},
	}

	// sources() -> list[str]
	env.Names["sources"] = &typecheck.Callable{
		Name:       "sources",
		ReturnType: &typecheck.List{Elem: typecheck.String},
	}

	// package is a string
	env.Names["package"] = typecheck.String

	// Library modules: json, sh, os

	// json module
	env.Names["json"] = &typecheck.Object{
		Name: "json",
		Attrs: map[string]typecheck.Type{
			"encode": &typecheck.Callable{
				Name:       "encode",
				Params:     []typecheck.Param{{Name: "x", Type: typecheck.Any}},
				ReturnType: typecheck.String,
			},
			"decode": &typecheck.Callable{
				Name:       "decode",
				Params:     []typecheck.Param{{Name: "x", Type: typecheck.String}},
				ReturnType: typecheck.Any,
			},
			"decode_all": &typecheck.Callable{
				Name:       "decode_all",
				Params:     []typecheck.Param{{Name: "x", Type: typecheck.String}},
				ReturnType: &typecheck.List{Elem: typecheck.Any},
			},
			"indent": &typecheck.Callable{
				Name: "indent",
				Params: []typecheck.Param{
					{Name: "x", Type: typecheck.String},
					{Name: "prefix", Type: typecheck.String, Optional: true},
					{Name: "indent", Type: typecheck.String, Optional: true},
				},
				ReturnType: typecheck.String,
			},
		},
	}

	// sh module
	env.Names["sh"] = &typecheck.Object{
		Name: "sh",
		Attrs: map[string]typecheck.Type{
			"exec": &typecheck.Callable{
				Name: "exec",
				Params: []typecheck.Param{
					{Name: "command", Type: typecheck.String},
					{Name: "cwd", Type: typecheck.String, Optional: true},
					{Name: "env", Type: &typecheck.Dict{Key: typecheck.String, Value: typecheck.String}, Optional: true},
					{Name: "try_", Type: typecheck.Bool, Optional: true},
				},
				ReturnType: typecheck.Any,
			},
			"output": &typecheck.Callable{
				Name: "output",
				Params: []typecheck.Param{
					{Name: "command", Type: typecheck.String},
					{Name: "cwd", Type: typecheck.String, Optional: true},
					{Name: "env", Type: &typecheck.Dict{Key: typecheck.String, Value: typecheck.String}, Optional: true},
					{Name: "try_", Type: typecheck.Bool, Optional: true},
				},
				ReturnType: typecheck.Any,
			},
		},
	}

	// os module
	osPathObj := &typecheck.Object{
		Name: "os.path",
		Attrs: map[string]typecheck.Type{
			"sep": typecheck.String,
			"is_abs": &typecheck.Callable{
				Name:       "is_abs",
				Params:     []typecheck.Param{{Name: "path", Type: typecheck.String}},
				ReturnType: typecheck.Bool,
			},
			"abs": &typecheck.Callable{
				Name:       "abs",
				Params:     []typecheck.Param{{Name: "path", Type: typecheck.String}},
				ReturnType: typecheck.String,
			},
			"base": &typecheck.Callable{
				Name:       "base",
				Params:     []typecheck.Param{{Name: "path", Type: typecheck.String}},
				ReturnType: typecheck.String,
			},
			"dir": &typecheck.Callable{
				Name:       "dir",
				Params:     []typecheck.Param{{Name: "path", Type: typecheck.String}},
				ReturnType: typecheck.String,
			},
			"join": &typecheck.Callable{
				Name:       "join",
				Params:     []typecheck.Param{{Name: "paths", Type: typecheck.String, Star: true}},
				ReturnType: typecheck.String,
			},
			"split": &typecheck.Callable{
				Name:       "split",
				Params:     []typecheck.Param{{Name: "path", Type: typecheck.String}},
				ReturnType: &typecheck.Tuple{Elems: []typecheck.Type{typecheck.String, typecheck.String}},
			},
			"splitext": &typecheck.Callable{
				Name:       "splitext",
				Params:     []typecheck.Param{{Name: "path", Type: typecheck.String}},
				ReturnType: &typecheck.Tuple{Elems: []typecheck.Type{typecheck.String, typecheck.String}},
			},
		},
	}

	env.Names["os"] = &typecheck.Object{
		Name: "os",
		Attrs: map[string]typecheck.Type{
			"path": osPathObj,
			"environ": &typecheck.Callable{
				Name:       "environ",
				ReturnType: &typecheck.Dict{Key: typecheck.String, Value: typecheck.String},
			},
			"look_path": &typecheck.Callable{
				Name:       "look_path",
				Params:     []typecheck.Param{{Name: "file", Type: typecheck.String}},
				ReturnType: &typecheck.Union{Types: []typecheck.Type{typecheck.String, typecheck.None}},
			},
			"exec": &typecheck.Callable{
				Name: "exec",
				Params: []typecheck.Param{
					{Name: "command", Type: &typecheck.List{Elem: typecheck.String}},
					{Name: "cwd", Type: typecheck.String, Optional: true},
					{Name: "env", Type: &typecheck.Dict{Key: typecheck.String, Value: typecheck.String}, Optional: true},
					{Name: "try_", Type: typecheck.Bool, Optional: true},
				},
				ReturnType: typecheck.Any,
			},
			"output": &typecheck.Callable{
				Name: "output",
				Params: []typecheck.Param{
					{Name: "command", Type: &typecheck.List{Elem: typecheck.String}},
					{Name: "cwd", Type: typecheck.String, Optional: true},
					{Name: "env", Type: &typecheck.Dict{Key: typecheck.String, Value: typecheck.String}, Optional: true},
					{Name: "try_", Type: typecheck.Bool, Optional: true},
				},
				ReturnType: typecheck.Any,
			},
			"exists": &typecheck.Callable{
				Name:       "exists",
				Params:     []typecheck.Param{{Name: "path", Type: typecheck.String}},
				ReturnType: typecheck.Bool,
			},
			"getcwd": &typecheck.Callable{
				Name:       "getcwd",
				ReturnType: typecheck.String,
			},
			"mkdir": &typecheck.Callable{
				Name: "mkdir",
				Params: []typecheck.Param{
					{Name: "path", Type: typecheck.String},
					{Name: "mode", Type: typecheck.Int, Optional: true},
				},
				ReturnType: typecheck.None,
			},
			"makedirs": &typecheck.Callable{
				Name: "makedirs",
				Params: []typecheck.Param{
					{Name: "path", Type: typecheck.String},
					{Name: "mode", Type: typecheck.Int, Optional: true},
				},
				ReturnType: typecheck.None,
			},
		},
	}

	// Type descriptors for named types

	// Cache type descriptor
	env.TypeDescriptors["Cache"] = &typecheck.TypeDescriptor{
		Methods: map[string]*typecheck.Callable{
			"once": {
				Name: "once",
				Params: []typecheck.Param{
					{Name: "key", Type: typecheck.String},
					{Name: "callable", Type: typecheck.Any},
				},
				ReturnType: typecheck.Any,
			},
		},
	}

	// Target type descriptor
	env.TypeDescriptors["Target"] = &typecheck.TypeDescriptor{
		Attrs: map[string]typecheck.Type{
			"label":        typecheck.String,
			"always":       typecheck.Bool,
			"function":     typecheck.Any,
			"dependencies": &typecheck.List{Elem: typecheck.String},
			"sources":      &typecheck.List{Elem: typecheck.String},
			"generates":    &typecheck.List{Elem: typecheck.String},
			"position":     typecheck.Any,
		},
	}

	return env
}

// isPredeclared returns a function that reports whether a name is predeclared
// in the given environment.
func isPredeclared(env *typecheck.Env) func(string) bool {
	return func(name string) bool {
		_, ok := env.Names[name]
		return ok
	}
}

// isUniversal reports whether a name is a Starlark universal (built-in).
// This matches the names in starlark.Universe plus struct and module.
func isUniversal(name string) bool {
	switch name {
	case "None", "True", "False",
		"abs", "any", "all", "bool", "bytes", "chr", "dict", "dir",
		"enumerate", "fail", "float", "getattr", "hasattr", "hash",
		"int", "len", "list", "max", "min", "ord", "print", "range",
		"repr", "reversed", "set", "sorted", "str", "tuple", "type", "zip",
		"struct", "module":
		return true
	}
	return false
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

// FileErrors holds the type-checking errors for a single file.
type FileErrors struct {
	Path   string
	Errors []typecheck.Error
}

// CheckProject walks a Dawn project root and type-checks all BUILD.dawn files.
func CheckProject(root string) ([]FileErrors, error) {
	// Load config to get ignore patterns.
	var ignore glob.Glob
	for _, name := range []string{"dawn.toml", ".dawnconfig"} {
		c, err := project.LoadConfigFile(filepath.Join(root, name))
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return nil, err
		}
		if len(c.Ignore) > 0 {
			g, err := glob.New(c.Ignore, nil)
			if err != nil {
				return nil, err
			}
			ignore = g
		}
		break
	}

	env := DawnEnv()

	var results []FileErrors
	err := walkProject(root, ".", ignore, func(path string) {
		src, err := os.ReadFile(path)
		if err != nil {
			results = append(results, FileErrors{
				Path:   path,
				Errors: []typecheck.Error{{Msg: err.Error()}},
			})
			return
		}
		errs := CheckFile(path, src, env)
		if len(errs) > 0 {
			results = append(results, FileErrors{Path: path, Errors: errs})
		}
	})
	if err != nil {
		return nil, err
	}
	return results, nil
}

func walkProject(root, rel string, ignore glob.Glob, fn func(path string)) error {
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
		if ignore != nil && ignore.MatchPath(filepath.ToSlash(childRel)) {
			continue
		}
		if err := walkProject(root, childRel, ignore, fn); err != nil {
			return err
		}
	}
	return nil
}
