package os

import (
	"os"
	"path/filepath"

	"github.com/pgavlin/dawn/util"
	"go.starlark.net/starlark"
)

// def exists(path):
//     """
//     Returns true if a file exists at the given path.
//     """
//
//starlark:builtin factory=NewExists,function=Exists
func exists(thread *starlark.Thread, fn *starlark.Builtin, path string) (starlark.Value, error) {
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

// def getcwd():
//     """
//     Returns the current OS working directory. This is typically the path of
//     the directory containg the root module on the callstack.
//     """
//
//starlark:builtin factory=NewGetcwd,function=Getcwd
func getcwd(thread *starlark.Thread, fn *starlark.Builtin) (starlark.Value, error) {
	return starlark.String(util.Getwd(thread)), nil
}
