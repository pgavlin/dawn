package os

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/pgavlin/dawn/util"
	"github.com/pgavlin/starlark-go/starlark"
)

// def glob(include, exclude=None):
//     """
//     Return a list of paths rooted in the current directory that match the
//     given include and exclude patterns.
//
//     - `*` matches any number of non-path-separator characters
//     - `**` matches any number of any characters
//     - `?` matches a single character
//
//     :param include: the patterns to include.
//     :param exclude: the patterns to exclude.
//
//     :returns: the matched paths
//     """
//
//starlark:builtin factory=NewGlob,function=Glob
func glob(t *starlark.Thread, fn *starlark.Builtin, include, exclude util.StringList) (starlark.Value, error) {
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
