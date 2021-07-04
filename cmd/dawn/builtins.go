// Code generated by dawn-gen-builtins; DO NOT EDIT.

package main


import (
	
	starlark "go.starlark.net/starlark"
	
)



func (w *workspace) newBuiltin_depends() *starlark.Builtin {
	const doc = `
   Returns the transitive closure of targets depended on by the given
   target.

   :param label_or_target: the label or target in question.
   :returns: the target's transitive dependency closure.
   :rtype: List[str]
   `
	return starlark.NewBuiltin("depends", w.starlark_builtin_depends).WithDoc(doc)
}


func (w *workspace) starlark_builtin_depends(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	
	var (
		
		labelOrTarget starlark.Value
		
	)
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "label_or_target", &labelOrTarget); err != nil {
		return nil, err
	}
	
	return w.builtin_depends(thread, fn, labelOrTarget)
}


func (w *workspace) newBuiltin_whatDepends() *starlark.Builtin {
	const doc = `
   Returns the transitive closure of target that depend on the given target.

   :param label_or_target: the label or target in question.
   :returns: the target's transitive dependent closure.
   :rtype: List[str]
   `
	return starlark.NewBuiltin("what_depends", w.starlark_builtin_whatDepends).WithDoc(doc)
}


func (w *workspace) starlark_builtin_whatDepends(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	
	var (
		
		labelOrTarget starlark.Value
		
	)
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "label_or_target", &labelOrTarget); err != nil {
		return nil, err
	}
	
	return w.builtin_whatDepends(thread, fn, labelOrTarget)
}


