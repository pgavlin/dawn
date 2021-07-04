package label

import (
	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
)

// starlark.Value
func (l *Label) Type() string {
	return "label"
}

func (l *Label) Freeze() {} // immutable

func (l *Label) Truth() starlark.Bool {
	return starlark.String(l.String()).Truth()
}

func (l *Label) Hash() (uint32, error) {
	return starlark.String(l.String()).Hash()
}

// starlark.Comparable
func (l *Label) CompareSameType(op syntax.Token, y starlark.Value, depth int) (bool, error) {
	return starlark.String(l.String()).CompareSameType(op, starlark.String(y.(*Label).String()), depth)
}

// starlark.HasAttrs
func (l *Label) Attr(name string) (starlark.Value, error) {
	switch name {
	case "kind":
		return starlark.String(l.Kind), nil
	case "module":
		return starlark.String(l.Module), nil
	case "version":
		if l.Version == nil {
			return starlark.None, nil
		}
		return starlark.String(l.Version.String()), nil
	case "package":
		return starlark.String(l.Package), nil
	case "name":
		return starlark.String(l.Name), nil
	default:
		return nil, nil
	}
}

func (l *Label) AttrNames() []string {
	return []string{"kind", "module", "version", "package", "name"}
}
