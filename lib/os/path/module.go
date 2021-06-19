package path

import (
	"path/filepath"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

var Module = &starlarkstruct.Module{
	Name: "path",
	Members: starlark.StringDict{
		"sep":      starlark.String(string([]rune{filepath.Separator})),
		"is_abs":   starlark.NewBuiltin("path.is_abs", IsAbs),
		"abs":      starlark.NewBuiltin("path.abs", Abs),
		"base":     starlark.NewBuiltin("path.base", Base),
		"dir":      starlark.NewBuiltin("path.dir", Dir),
		"join":     starlark.NewBuiltin("path.join", Join),
		"split":    starlark.NewBuiltin("path.split", Split),
		"splitext": starlark.NewBuiltin("path.splitext", Splitext),
	},
}
