package dawn

import "github.com/pgavlin/starlark-go/typecheck"

// cacheType is the Named type for Cache values.
var cacheType = typecheck.NewNamed("Cache", typecheck.WithAttrs(map[string]typecheck.Type{
	"once": &typecheck.Callable{
		Name: "once",
		Params: []typecheck.Param{
			{Name: "key", Type: typecheck.String},
			{Name: "callable", Type: typecheck.Any},
		},
		ReturnType: typecheck.Any,
	},
}))

// targetType is the Named type for Target values.
var targetType = typecheck.NewNamed("Target", typecheck.WithAttrs(map[string]typecheck.Type{
	"label":        typecheck.String,
	"always":       typecheck.Bool,
	"function":     typecheck.Any,
	"dependencies": &typecheck.List{Elem: typecheck.String},
	"sources":      &typecheck.List{Elem: typecheck.String},
	"generates":    &typecheck.List{Elem: typecheck.String},
	"position":     typecheck.Any,
}))

// DawnEnv returns a typecheck.Env that includes all Dawn-specific predeclared
// names. Universal builtins live in typecheck.Universe.
func DawnEnv() *typecheck.Env {
	env := &typecheck.Env{
		Predeclared: make(map[string]typecheck.Type),
	}

	// struct and module constructors (universals in Dawn)
	env.Predeclared["struct"] = &typecheck.Callable{
		Name:       "struct",
		Params:     []typecheck.Param{{Name: "kwargs", Type: typecheck.Any, StarStar: true}},
		ReturnType: typecheck.Any,
	}
	env.Predeclared["module"] = &typecheck.Callable{
		Name:       "module",
		Params:     []typecheck.Param{{Name: "name", Type: typecheck.String}, {Name: "kwargs", Type: typecheck.Any, StarStar: true}},
		ReturnType: typecheck.Any,
	}

	// host object
	env.Predeclared["host"] = &typecheck.Object{
		Name: "host",
		Attrs: map[string]typecheck.Type{
			"arch": typecheck.String,
			"os":   typecheck.String,
		},
	}

	// Cache constructor
	env.Predeclared["Cache"] = &typecheck.Callable{
		Name:       "Cache",
		ReturnType: cacheType,
	}

	// path(label: str) -> str
	env.Predeclared["path"] = &typecheck.Callable{
		Name:       "path",
		Params:     []typecheck.Param{{Name: "label", Type: typecheck.String}},
		ReturnType: typecheck.String,
	}

	// label(path: str) -> str
	env.Predeclared["label"] = &typecheck.Callable{
		Name:       "label",
		Params:     []typecheck.Param{{Name: "path", Type: typecheck.String}},
		ReturnType: typecheck.String,
	}

	// contains(path: str) -> tuple[str | None, bool]
	env.Predeclared["contains"] = &typecheck.Callable{
		Name:   "contains",
		Params: []typecheck.Param{{Name: "path", Type: typecheck.String}},
		ReturnType: &typecheck.Tuple{Elems: []typecheck.Type{
			&typecheck.Union{Types: []typecheck.Type{typecheck.String, typecheck.None}},
			typecheck.Bool,
		}},
	}

	// parse_flag(name, default=None, type=None, choices=None, required=None, help=None) -> any
	env.Predeclared["parse_flag"] = &typecheck.Callable{
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
	env.Predeclared["target"] = &typecheck.Callable{
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
		ReturnType: targetType,
	}

	// glob(include, exclude=None, dirs=None) -> list[str]
	stringOrStringList := &typecheck.Union{Types: []typecheck.Type{
		typecheck.String,
		&typecheck.List{Elem: typecheck.String},
	}}
	env.Predeclared["glob"] = &typecheck.Callable{
		Name: "glob",
		Params: []typecheck.Param{
			{Name: "include", Type: stringOrStringList},
			{Name: "exclude", Type: stringOrStringList, Optional: true},
			{Name: "dirs", Type: typecheck.Bool, Optional: true},
		},
		ReturnType: &typecheck.List{Elem: typecheck.String},
	}

	// fail(message: str) -> None
	env.Predeclared["fail"] = &typecheck.Callable{
		Name:       "fail",
		Params:     []typecheck.Param{{Name: "message", Type: typecheck.String}},
		ReturnType: typecheck.None,
	}

	// run(label_or_target, always=None, dry_run=None, callback=None) -> None
	env.Predeclared["run"] = &typecheck.Callable{
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
	env.Predeclared["get_target"] = &typecheck.Callable{
		Name:       "get_target",
		Params:     []typecheck.Param{{Name: "label", Type: typecheck.String}},
		ReturnType: targetType,
	}

	// flags() -> list[Flag]
	env.Predeclared["flags"] = &typecheck.Callable{
		Name:       "flags",
		ReturnType: &typecheck.List{Elem: typecheck.NewNamed("Flag")},
	}

	// targets() -> list[Target]
	env.Predeclared["targets"] = &typecheck.Callable{
		Name:       "targets",
		ReturnType: &typecheck.List{Elem: targetType},
	}

	// sources() -> list[str]
	env.Predeclared["sources"] = &typecheck.Callable{
		Name:       "sources",
		ReturnType: &typecheck.List{Elem: typecheck.String},
	}

	// package is a string
	env.Predeclared["package"] = typecheck.String

	// Library modules: json, sh, os

	// json module
	env.Predeclared["json"] = &typecheck.Object{
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
	env.Predeclared["sh"] = &typecheck.Object{
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

	env.Predeclared["os"] = &typecheck.Object{
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

	return env
}

// isPredeclared returns a function that reports whether a name is predeclared
// in the given environment.
func isPredeclared(env *typecheck.Env) func(string) bool {
	return func(name string) bool {
		_, ok := env.Predeclared[name]
		return ok
	}
}

// isUniversal reports whether a name is a Starlark universal (built-in).
func isUniversal(name string) bool {
	_, ok := typecheck.Universe[name]
	if ok {
		return true
	}
	// struct and module are also treated as universals in Dawn
	switch name {
	case "struct", "module":
		return true
	}
	return false
}
