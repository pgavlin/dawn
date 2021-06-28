package os

import (
	"github.com/pgavlin/dawn/lib/os/path"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// def os():
//     """
//     Provides a platform-independent interface to host operating system
//     functionality. Functions in this package expect and return host paths.
//     """
//
//     @function("execf")
//     def exec():
//         pass
//
//     @function("output")
//     def output():
//         pass
//
//     @function("exists")
//     def exists():
//         pass
//
//     @function("getcwd")
//     def getcwd():
//         pass
//
//     @function("glob")
//     def glob():
//         pass
//
//starlark:module
var Module = &starlarkstruct.Module{
	Name: "os",
	Members: starlark.StringDict{
		"path": path.Module,

		"exec":   NewExec(),
		"output": NewOutput(),
		"exists": NewExists(),
		"getcwd": NewGetcwd(),
		"glob":   NewGlob(),
	},
}
