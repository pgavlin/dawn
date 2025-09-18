package path

import (
	"fmt"
	"path/filepath"

	"github.com/pgavlin/dawn/util"
	"github.com/pgavlin/starlark-go/starlark"
)

// starlark
//
//	def abs(path):
//	    """
//	    Returns an absolute representation of path. If the path is not absolute
//	    it will be joined with the current working directory (usually the
//	    directory containing the root module on the stack) to turn it into an
//	    absolute path.
//	    """
//
//starlark:builtin factory=NewAbs,function=Abs
func abs(thread *starlark.Thread, fn *starlark.Builtin, path string) (starlark.Value, error) {
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

// starlark
//
//	def is_abs(path):
//	    """
//	    Returns True if path is absolute.
//	    """
//
//starlark:builtin factory=NewIsAbs,function=IsAbs
func isAbs(thread *starlark.Thread, fn *starlark.Builtin, path string) (starlark.Value, error) {
	return starlark.Bool(filepath.IsAbs(path)), nil
}

// starlark
//
//	def base(path):
//	    """
//	    Returns the last element of path. Trailing path separators are removed
//	    before extracting the last element. If the path is empty, Base returns
//	    ".". If the path consists entirely of separators, Base returns a single
//	    separator.
//	    """
//
//starlark:builtin factory=NewBase,function=Base
func base(thread *starlark.Thread, fn *starlark.Builtin, path string) (starlark.Value, error) {
	return starlark.String(filepath.Base(path)), nil
}

// starlark
//
//	def dir(path):
//	    """
//	    Returns all but the last element of path. If the path is empty, dir
//	    returns ".". If the path consists entirely of separators, dir returns a
//	    single separator. The returned path does not end in a separator unless
//	    it is the root directory.
//	    """
//
//starlark:builtin factory=NewDir,function=Dir
func dir(thread *starlark.Thread, fn *starlark.Builtin, path string) (starlark.Value, error) {
	return starlark.String(filepath.Dir(path)), nil
}

// starlark
//
//	def join(components):
//	    """
//	    Joins any number of path elements into a single path, separating them
//	    with a host-specific separator. Empty elements are ignored.
//	    """
//
//starlark:builtin factory=NewJoin,function=Join
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

// starlark
//
//	def split(path):
//	    """
//	    Splits path immediately following the final separator, separating it into
//	    a directory and file name component. If there is no separator in path,
//	    split returns an empty dir and file set to path.
//	    """
//
//starlark:builtin factory=NewSplit,function=Split
func split(thread *starlark.Thread, fn *starlark.Builtin, path string) (starlark.Value, error) {
	dir, file := filepath.Split(path)
	return starlark.Tuple{starlark.String(dir), starlark.String(file)}, nil
}

// starlark
//
//	def splitext(path):
//	    """
//	    Splits the pathname path into a pair (root, ext) such that
//	    root + ext == path, and ext is empty or begins with a period and contains
//	    at most one period. Leading periods on the basename are ignored;
//	    splitext('.cshrc') returns ('.cshrc', '').
//	    """
//
//starlark:builtin factory=NewSplitext,function=Splitext
func splitext(thread *starlark.Thread, fn *starlark.Builtin, path string) (starlark.Value, error) {
	ext := filepath.Ext(path)
	if ext == path {
		return starlark.Tuple{starlark.String(path), starlark.String("")}, nil
	}
	return starlark.Tuple{starlark.String(path[:len(path)-len(ext)]), starlark.String(ext)}, nil
}
