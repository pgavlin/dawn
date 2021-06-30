//go:generate go run github.com/pgavlin/dawn/cmd/dawn-gen-builtins . builtins.go ../../../docs/source/modules

package path

import (
	"path/filepath"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// def path():
//     """
//     The path module provides utility functions for manipulating host-specific
//     paths. This module uses either forward- or backwards-facing slashes for
//     separating path components, depending on the host operating system.
//     """
//
//     @attribute
//     def sep():
//         """
//         The host-specific path separator.
//         """
//
//     @function("isAbs")
//     def is_abs():
//         pass
//
//     @function("abs")
//     def abs():
//         pass
//
//     @function("base")
//     def base():
//         pass
//
//     @function("dir")
//     def dir():
//         pass
//
//     @function("Join")
//     def join():
//         pass
//
//     @function("split")
//     def split():
//         pass
//
//     @function("splitext")
//     def splitext():
//         pass
//
//starlark:module
var Module = &starlarkstruct.Module{
	Name: "path",
	Members: starlark.StringDict{
		"sep":      starlark.String(string([]rune{filepath.Separator})),
		"is_abs":   NewIsAbs(),
		"abs":      NewAbs(),
		"base":     NewBase(),
		"dir":      NewDir(),
		"join":     NewJoin(),
		"split":    NewSplit(),
		"splitext": NewSplitext(),
	},
}
