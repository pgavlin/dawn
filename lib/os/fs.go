package os

import (
	"os"
	"path/filepath"

	"github.com/pgavlin/dawn/util"
	"go.starlark.net/starlark"
)

func Exists(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "path", &path); err != nil {
		return nil, err
	}

	cwd := util.Getwd(thread)
	if !filepath.IsAbs(path) {
		path = filepath.Join(cwd, path)
	}

	_, err := os.Stat(path)
	switch {
	case err == nil:
		return starlark.True, nil
	case os.IsNotExist(err):
		return starlark.False, nil
	default:
		return nil, err
	}
}

func Getcwd(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackPositionalArgs(fn.Name(), args, kwargs, 0); err != nil {
		return nil, err
	}

	return starlark.String(util.Getwd(thread)), nil
}
