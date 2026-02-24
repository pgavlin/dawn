package dawn

import (
	"github.com/pgavlin/starlark-go/starlark"
	"github.com/pgavlin/starlark-go/typecheck"
)

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

// staticType derives a typecheck.Type from a runtime starlark.Value.
//
// For HasAttrs values (modules, structs), it recursively builds an Object type.
// For builtins and functions, it delegates to starlark.StaticType which returns
// a Callable with parameter metadata. For all other values, starlark.StaticType
// handles basic types, containers, etc.
func staticType(v starlark.Value) typecheck.Type {
	if ha, ok := v.(starlark.HasAttrs); ok {
		// Builtins and Functions implement HasAttrs (for method access),
		// but should use their callable signature from starlark.StaticType.
		switch v.(type) {
		case *starlark.Builtin, *starlark.Function:
			// fall through to StaticType
		default:
			attrs := make(map[string]typecheck.Type)
			for _, name := range ha.AttrNames() {
				if a, err := ha.Attr(name); err == nil && a != nil {
					attrs[name] = staticType(a)
				}
			}
			return &typecheck.Object{Name: v.Type(), Attrs: attrs}
		}
	}
	return starlark.StaticType(v)
}

// buildTypeEnv constructs a typecheck.Env from the project's runtime builtins,
// mirroring the predeclared names assembled by module.env().
func (proj *Project) buildTypeEnv() *typecheck.Env {
	env := &typecheck.Env{Predeclared: make(map[string]typecheck.Type)}

	// Project-provided builtins (json, sh, os, etc.)
	for k, v := range proj.builtins {
		env.Predeclared[k] = staticType(v)
	}

	// Module-local builtins (same set that module.env assembles)
	env.Predeclared["host"] = staticType(builtin_host)
	env.Predeclared["Cache"] = staticType(builtin_cache)
	env.Predeclared["path"] = staticType(proj.newBuiltin_path())
	env.Predeclared["label"] = staticType(proj.newBuiltin_label())
	env.Predeclared["contains"] = staticType(proj.newBuiltin_contains())
	env.Predeclared["parse_flag"] = staticType(proj.newBuiltin_parse_flag())
	env.Predeclared["target"] = staticType(proj.newBuiltin_target())
	env.Predeclared["glob"] = staticType(proj.newBuiltin_glob())
	env.Predeclared["fail"] = staticType(proj.newBuiltin_fail())

	// package is always a string
	env.Predeclared["package"] = typecheck.String

	// struct and module constructors — treated as universals by isUniversal
	// but must be in Predeclared so the type checker can resolve their types
	// (they are not in typecheck.Universe).
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

	return env
}

// IsPredeclared returns a function that reports whether a name is predeclared
// in the given environment.
func IsPredeclared(env *typecheck.Env) func(string) bool {
	return func(name string) bool {
		_, ok := env.Predeclared[name]
		return ok
	}
}

// IsUniversal reports whether a name is a Starlark universal (built-in).
func IsUniversal(name string) bool {
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

// BuildTypeEnv constructs a typecheck.Env from the given builtins.
// This is useful for creating an env without a fully-initialized Project.
func BuildTypeEnv(builtins starlark.StringDict) *typecheck.Env {
	proj := &Project{builtins: builtins}
	return proj.buildTypeEnv()
}
