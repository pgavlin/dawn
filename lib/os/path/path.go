package path

import (
	"fmt"
	"path/filepath"

	"github.com/pgavlin/dawn/util"
	"go.starlark.net/starlark"
)

func Abs(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "path", &path); err != nil {
		return nil, err
	}

	cwd := util.Getwd(thread)
	if !filepath.IsAbs(path) {
		path = filepath.Join(cwd, path)
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	return starlark.String(abs), nil
}

func IsAbs(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "path", &path); err != nil {
		return nil, err
	}

	return starlark.Bool(filepath.IsAbs(path)), nil
}

func Base(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "path", &path); err != nil {
		return nil, err
	}

	return starlark.String(filepath.Base(path)), nil
}

func Dir(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "path", &path); err != nil {
		return nil, err
	}

	return starlark.String(filepath.Dir(path)), nil
}

func Join(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if len(kwargs) != 0 {
		return nil, fmt.Errorf("%v: unexpected keyword args", fn.Name())
	}

	var components util.StringList
	if err := components.Unpack(args); err != nil {
		return nil, err
	}

	return starlark.String(filepath.Join([]string(components)...)), nil
}

func Split(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "path", &path); err != nil {
		return nil, err
	}

	return util.StringList(filepath.SplitList(path)).List(), nil
}

func Splitext(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "path", &path); err != nil {
		return nil, err
	}

	ext := filepath.Ext(path)
	if ext == path {
		return starlark.Tuple{starlark.String(path), starlark.String("")}, nil
	}
	return starlark.Tuple{starlark.String(path[:len(path)-len(ext)]), starlark.String(ext)}, nil
}
