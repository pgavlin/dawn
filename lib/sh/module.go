package sh

import (
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// def sh():
//     """
//     The sh module provides functions for executing POSIX Shell, Bash, and
//     mksh commands. The implementation uses the mvdan.cc/sh interpreter,
//     instead of relying on the system shell, and therefore provides a
//     consistent experience across all platforms (including Windows).
//     """
//
//     @function("exec")
//     def exec():
//         pass
//
//     @function("output")
//     def output():
//         pass
//
//starlark:module
type module int

var Module = &starlarkstruct.Module{
	Name: "sh",
	Members: starlark.StringDict{
		"exec":   NewExec(),
		"output": NewOutput(),
	},
}
