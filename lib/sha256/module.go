package sha256

import (
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

var Module = &starlarkstruct.Module{
	Name: "sha256",
	Members: starlark.StringDict{
		"hash": starlark.NewBuiltin("sha256.hash", Hash),
		"file": starlark.NewBuiltin("sha256.file", File),
	},
}
