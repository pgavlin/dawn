package util

import (
	"errors"

	"github.com/pgavlin/fx/v2"
	fxs "github.com/pgavlin/fx/v2/slices"
	"github.com/pgavlin/starlark-go/starlark"
)

type StringList []string

func (l StringList) List() *starlark.List {
	vs := make([]starlark.Value, len(l))
	for i, s := range l {
		vs[i] = starlark.String(s)
	}
	return starlark.NewList(vs)
}

func (l *StringList) Unpack(v starlark.Value) error {
	if seq, ok := v.(starlark.Sequence); ok {
		strings, err := fxs.TryCollect(fx.MapUnpack(All(seq), func(v starlark.Value) (string, error) {
			s, ok := starlark.AsString(v)
			if !ok {
				return "", errors.New("expected a sequence of strings")
			}
			return s, nil
		}))
		if err != nil {
			return err
		}
		*l = strings
		return nil
	}
	if s, ok := starlark.AsString(v); ok {
		*l = StringList{s}
		return nil
	}
	return errors.New("expected a sequence of strings or a single string")
}
