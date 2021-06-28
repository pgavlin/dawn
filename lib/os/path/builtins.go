// Code generated by dawn-gen-builtins; DO NOT EDIT.

package path


import (
	
	starlark "go.starlark.net/starlark"
	
)



func NewAbs() *starlark.Builtin {
	const doc = `
   Returns an absolute representation of path. If the path is not absolute
   it will be joined with the current working directory (usually the
   directory containing the root module on the stack) to turn it into an
   absolute path.
   `
	return starlark.NewBuiltin("abs", Abs).WithDoc(doc)
}


func Abs(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	
	var (
		
		path string
		
	)
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "path", &path); err != nil {
		return nil, err
	}
	
	return abs(thread, fn, path)
}


func NewIsAbs() *starlark.Builtin {
	const doc = `
   Returns True if path is absolute.
   `
	return starlark.NewBuiltin("is_abs", IsAbs).WithDoc(doc)
}


func IsAbs(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	
	var (
		
		path string
		
	)
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "path", &path); err != nil {
		return nil, err
	}
	
	return isAbs(thread, fn, path)
}


func NewBase() *starlark.Builtin {
	const doc = `
   Returns the last element of path. Trailing path separators are removed
   before extracting the last element. If the path is empty, Base returns
   ".". If the path consists entirely of separators, Base returns a single
   separator.
   `
	return starlark.NewBuiltin("base", Base).WithDoc(doc)
}


func Base(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	
	var (
		
		path string
		
	)
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "path", &path); err != nil {
		return nil, err
	}
	
	return base(thread, fn, path)
}


func NewDir() *starlark.Builtin {
	const doc = `
   Returns all but the last element of path. If the path is empty, dir
   returns ".". If the path consists entirely of separators, dir returns a
   single separator. The returned path does not end in a separator unless
   it is the root directory.
   `
	return starlark.NewBuiltin("dir", Dir).WithDoc(doc)
}


func Dir(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	
	var (
		
		path string
		
	)
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "path", &path); err != nil {
		return nil, err
	}
	
	return dir(thread, fn, path)
}


func NewJoin() *starlark.Builtin {
	const doc = `
   Joins any number of path elements into a single path, separating them
   with a host-specific separator. Empty elements are ignored.
   `
	return starlark.NewBuiltin("join", Join).WithDoc(doc)
}



func NewSplit() *starlark.Builtin {
	const doc = `
   Splits path immediately following the final separator, separating it into
   a directory and file name component. If there is no separator in path,
   split returns an empty dir and file set to path.
   `
	return starlark.NewBuiltin("split", Split).WithDoc(doc)
}


func Split(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	
	var (
		
		path string
		
	)
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "path", &path); err != nil {
		return nil, err
	}
	
	return split(thread, fn, path)
}


func NewSplitext() *starlark.Builtin {
	const doc = `
   Splits the pathname path into a pair (root, ext) such that
   root + ext == path, and ext is empty or begins with a period and contains
   at most one period. Leading periods on the basename are ignored;
   splitext('.cshrc') returns ('.cshrc', '').
   `
	return starlark.NewBuiltin("splitext", Splitext).WithDoc(doc)
}


func Splitext(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	
	var (
		
		path string
		
	)
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "path", &path); err != nil {
		return nil, err
	}
	
	return splitext(thread, fn, path)
}


