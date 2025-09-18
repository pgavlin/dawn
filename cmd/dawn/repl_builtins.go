package main

import (
	"fmt"

	"github.com/pgavlin/dawn"
	"github.com/pgavlin/dawn/label"
	"github.com/pgavlin/dawn/util"
	"github.com/pgavlin/starlark-go/starlark"
)

func getLabel(thread *starlark.Thread, fn *starlark.Builtin, labelOrTarget starlark.Value) (*label.Label, error) {
	switch labelOrTarget := labelOrTarget.(type) {
	case starlark.String:
		l, err := label.Parse(string(labelOrTarget))
		if err != nil {
			return nil, err
		}

		m, ok := dawn.CurrentModule(thread)
		if !ok && !l.IsAbs() {
			return nil, fmt.Errorf("%v: no current module; label %v must be absolute", fn.Name(), l)
		}

		return l.RelativeTo(m.Package)
	case dawn.Target:
		return labelOrTarget.Label(), nil
	default:
		return nil, fmt.Errorf("%v: label_or_target must be a string or a target", fn.Name())
	}
}

// def depends(label_or_target):
//     """
//     Returns the transitive closure of targets depended on by the given
//     target.
//
//     :param label_or_target: the label or target in question.
//     :returns: the target's transitive dependency closure.
//     :rtype: List[str]
//     """
//
//starlark:builtin
func (w *workspace) builtin_depends(thread *starlark.Thread, fn *starlark.Builtin, labelOrTarget starlark.Value) (_ starlark.Value, err error) {
	label, err := getLabel(thread, fn, labelOrTarget)
	if err != nil {
		return nil, err
	}

	list, err := w.depends(label)
	if err != nil {
		return nil, err
	}
	return util.StringList(list).List(), nil
}

// def what_depends(label_or_target):
//     """
//     Returns the transitive closure of target that depend on the given target.
//
//     :param label_or_target: the label or target in question.
//     :returns: the target's transitive dependent closure.
//     :rtype: List[str]
//     """
//
//starlark:builtin
func (w *workspace) builtin_whatDepends(thread *starlark.Thread, fn *starlark.Builtin, labelOrTarget starlark.Value) (_ starlark.Value, err error) {
	label, err := getLabel(thread, fn, labelOrTarget)
	if err != nil {
		return nil, err
	}

	list, err := w.whatDepends(label)
	if err != nil {
		return nil, err
	}
	return util.StringList(list).List(), nil
}

// def cli():
//     """
//     CLI-only builtin functions.
//     """
//
//     @function("*workspace.builtin_depends")
//     def depends():
//         pass
//
//     @function("*workspace.builtin_whatDepends")
//     def what_depends():
//         pass
//
//starlark:module
type cliModule int
