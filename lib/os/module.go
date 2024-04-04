//go:generate go run github.com/pgavlin/dawn/cmd/dawn-gen-builtins . builtins.go ../../docs/source/modules

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
//     @module
//     def path():
//         """
//         The path module provides functions to manipulate host paths.
//         """
//
//     @function("environ")
//     def environ():
//         pass
//
//     @function("lookPath")
//     def look_path():
//         pass
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
//     @function("mkdir")
//     def mkdir():
//         pass
//
//     @function("makedirs")
//     def makedirs():
//         pass
//
//starlark:module
var Module = &starlarkstruct.Module{
	Name: "os",
	Members: starlark.StringDict{
		"path": path.Module,

		"environ":   NewEnviron(),
		"look_path": NewLookPath(),
		"exec":      NewExec(),
		"output":    NewOutput(),
		"exists":    NewExists(),
		"getcwd":    NewGetcwd(),
		"glob":      NewGlob(),
		"mkdir":     NewMkdir(),
		"makedirs":  NewMakedirs(),
	},
}
