package util

import (
	"errors"

	"go.starlark.net/starlark"
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
	seq, ok := v.(starlark.Sequence)
	if !ok {
		return errors.New("expected a sequence of strings")
	}

	var strings []string
	it := seq.Iterate()
	defer it.Done()
	for it.Next(&v) {
		s, ok := starlark.AsString(v)
		if !ok {
			return errors.New("expected a sequence of strings")
		}
		strings = append(strings, s)
	}
	*l = strings
	return nil
}
