package diff

import (
	"go.starlark.net/starlark"
)

func Diff(old, new starlark.Value) (ValueDiff, error) {
	return DiffDepth(old, new, starlark.CompareLimit)
}

func DiffDepth(old, new starlark.Value, depth int) (ValueDiff, error) {
	eq, err := starlark.EqualDepth(old, new, depth)
	if err != nil {
		return nil, err
	}
	if eq {
		return nil, nil
	}

	oldSlice, oldIsSlice := old.(starlark.Sliceable)
	newSlice, newIsSlice := new.(starlark.Sliceable)
	if oldIsSlice && newIsSlice {
		return diffSlice(oldSlice, newSlice, depth-1)
	}

	oldMapping, oldIsMapping := old.(starlark.IterableMapping)
	newMapping, newIsMapping := new.(starlark.IterableMapping)
	if oldIsMapping && newIsMapping {
		return diffMapping(oldMapping, newMapping, depth-1)
	}

	return &LiteralDiff{valueDiff: valueDiff{old: old, new: new}}, nil
}

func diffMapping(old, new starlark.IterableMapping, depth int) (*MappingDiff, error) {
	add := starlark.NewDict(0)
	edit := starlark.NewDict(0)
	remove := starlark.NewDict(0)

	oldKeys := old.Iterate()
	defer oldKeys.Done()

	var key starlark.Value
	for oldKeys.Next(&key) {
		oldV, _, _ := old.Get(key)
		newV, has, _ := new.Get(key)
		if !has {
			remove.SetKey(key, oldV)
			continue
		}

		diff, err := DiffDepth(oldV, newV, depth)
		if err != nil {
			return nil, err
		}
		if diff != nil {
			edit.SetKey(key, diff)
		}
	}

	newKeys := new.Iterate()
	defer newKeys.Done()

	for newKeys.Next(&key) {
		if _, has, _ := old.Get(key); !has {
			newV, _, _ := new.Get(key)
			add.SetKey(key, newV)
		}
	}

	return &MappingDiff{
		valueDiff: valueDiff{old: old, new: new},
		add:       add,
		edit:      edit,
		remove:    remove,
	}, nil
}
