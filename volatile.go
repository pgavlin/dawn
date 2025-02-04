package dawn

import (
	"fmt"

	"github.com/pgavlin/starlark-go/starlark"
)

// A volatile provides a wrapper around an inner value that allows targets to ignore irrelevant changes.
type volatile struct {
	v starlark.Value
}

var builtin_volatile = starlark.NewBuiltin("Volatile", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var v starlark.Value
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "value", &v); err != nil {
		return nil, err
	}

	return &volatile{v}, nil
})

func (v *volatile) String() string        { return fmt.Sprintf("Volatile(%v)", v.v.String()) }
func (v *volatile) Type() string          { return fmt.Sprintf("volatile[%v]", v.v.Type()) }
func (v *volatile) Freeze()               { v.v.Freeze() }
func (v *volatile) Truth() starlark.Bool  { return v.v.Truth() }
func (v *volatile) Hash() (uint32, error) { return v.v.Hash() }

func (v *volatile) Attr(name string) (starlark.Value, error) {
	if name != "value" {
		return nil, nil
	}
	return v.v, nil
}

func (v *volatile) AttrNames() []string {
	return []string{"value"}
}
