package os

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/pgavlin/dawn/util"
	"go.starlark.net/starlark"
)

func Glob(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		include util.StringList
		exclude util.StringList
	)

	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "include", &include, "exclude?", &exclude); err != nil {
		return nil, err
	}

	includeRE, err := util.CompileGlobs([]string(include))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", fn.Name(), err)
	}

	excludeRE, err := util.CompileGlobs([]string(exclude))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", fn.Name(), err)
	}

	dir := util.Getwd(t)

	var matches []starlark.Value
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		switch {
		case path == dir:
			return nil
		case len(path) > len(dir) && path[:len(dir)] == dir && path[len(dir)] == os.PathSeparator:
			path = path[len(dir)+1:]
		}
		if includeRE.MatchString(path) && !excludeRE.MatchString(path) {
			matches = append(matches, starlark.String(path))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return starlark.NewList(matches), nil
}
