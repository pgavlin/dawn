package sh

import (
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

var Module = &starlarkstruct.Module{
	Name: "sh",
	Members: starlark.StringDict{
		"exec":   starlark.NewBuiltin("sh.exec", Exec),
		"output": starlark.NewBuiltin("sh.output", Output),
	},
}
