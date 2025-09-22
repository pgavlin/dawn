package diff

import (
	"github.com/pgavlin/dawn/util"
	"github.com/pgavlin/starlark-go/starlark"
)

// Diff diffs two values.
func Diff(old, new starlark.Value) (ValueDiff, error) {
	return DiffDepth(old, new, starlark.CompareLimit)
}

// DiffDepth diffs two values up to the given depth.
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
	edits := starlark.NewDict(0)

	for key := range util.All(old) {
		oldV, _, _ := old.Get(key)
		newV, has, _ := new.Get(key)
		if !has {
			util.Must(edits.SetKey(key, &Edit{Sliceable: starlark.Tuple{oldV}, kind: EditKindDelete}))
			continue
		}

		diff, err := DiffDepth(oldV, newV, depth)
		if err != nil {
			return nil, err
		}
		if diff != nil {
			util.Must(edits.SetKey(key, &Edit{Sliceable: starlark.Tuple{diff}, kind: EditKindReplace}))
		}
	}

	for key := range util.All(new) {
		if _, has, _ := old.Get(key); !has {
			newV, _, _ := new.Get(key)
			util.Must(edits.SetKey(key, &Edit{Sliceable: starlark.Tuple{newV}, kind: EditKindAdd}))
		}
	}

	return &MappingDiff{
		valueDiff: valueDiff{old: old, new: new},
		edits:     edits,
	}, nil
}
