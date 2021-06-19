package os

import (
	"github.com/pgavlin/dawn/lib/os/path"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

var Module = &starlarkstruct.Module{
	Name: "os",
	Members: starlark.StringDict{
		"path": path.Module,

		"exec":   starlark.NewBuiltin("os.exec", Exec),
		"output": starlark.NewBuiltin("os.output", Output),
		"exists": starlark.NewBuiltin("os.exists", Exists),
		"getcwd": starlark.NewBuiltin("os.getcwd", Getcwd),
		"glob":   starlark.NewBuiltin("os.glob", Glob),
	},
}
