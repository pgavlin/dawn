package diff

import (
	"fmt"
	"strings"

	"go.starlark.net/starlark"
)

type ValueDiff interface {
	starlark.Value

	Old() starlark.Value
	New() starlark.Value
}

type valueDiff struct {
	old, new starlark.Value
}

func (d *valueDiff) freeze() {
	d.old.Freeze()
	d.new.Freeze()
}

func (d *valueDiff) Truth() starlark.Bool {
	return true
}

func (d *valueDiff) Hash() (uint32, error) {
	return 0, fmt.Errorf("unhashable type: diff")
}

func (d *valueDiff) Old() starlark.Value {
	return d.old
}

func (d *valueDiff) New() starlark.Value {
	return d.new
}

type LiteralDiff struct {
	valueDiff
}

func (d *LiteralDiff) String() string {
	return fmt.Sprintf("(%v -> %v)", d.Old(), d.New())
}

func (d *LiteralDiff) Type() string {
	return "LiteralDiff"
}

func (d *LiteralDiff) Freeze() {
	d.freeze()
}

func (d *LiteralDiff) Attr(name string) (starlark.Value, error) {
	switch name {
	case "old":
		return d.old, nil
	case "new":
		return d.new, nil
	default:
		return nil, nil
	}
}

func (d *LiteralDiff) AttrNames() []string {
	return []string{"old", "new"}
}

type MappingDiff struct {
	valueDiff

	add    *starlark.Dict
	edit   *starlark.Dict
	remove *starlark.Dict
}

func (d *MappingDiff) String() string {
	return fmt.Sprintf("{+%v, ~%v, -%v}", d.Add(), d.Edit(), d.Remove())
}

func (d *MappingDiff) Type() string {
	return "MappingDiff"
}

func (d *MappingDiff) Freeze() {
	d.freeze()
	d.add.Freeze()
	d.edit.Freeze()
	d.remove.Freeze()
}

func (d *MappingDiff) Attr(name string) (starlark.Value, error) {
	switch name {
	case "old":
		return d.old, nil
	case "new":
		return d.new, nil
	case "add":
		return d.add, nil
	case "edit":
		return d.edit, nil
	case "remove":
		return d.remove, nil
	default:
		return nil, nil
	}
}

func (d *MappingDiff) AttrNames() []string {
	return []string{"old", "new", "add", "edit", "remove"}
}

func (d *MappingDiff) Add() starlark.IterableMapping {
	return d.add
}

func (d *MappingDiff) Edit() starlark.IterableMapping {
	return d.edit
}

func (d *MappingDiff) Remove() starlark.IterableMapping {
	return d.remove
}

type EditKind = starlark.String

var (
	EditKindDelete  EditKind = "delete"
	EditKindCommon  EditKind = "common"
	EditKindAdd     EditKind = "add"
	EditKindReplace EditKind = "replace"
)

type Edit struct {
	starlark.Sliceable

	kind EditKind
}

func (e *Edit) String() string {
	r := '='
	switch e.Kind() {
	case EditKindDelete:
		r = '-'
	case EditKindAdd:
		r = '+'
	case EditKindReplace:
		r = '~'
	}
	return fmt.Sprintf("%c%v", r, e.Sliceable)
}

func (e *Edit) Type() string {
	return "Edit"
}

func (e *Edit) Attr(name string) (starlark.Value, error) {
	if name == "kind" {
		return e.kind, nil
	}
	return nil, nil
}

func (d *Edit) AttrNames() []string {
	return []string{"kind"}
}

func (e *Edit) Kind() EditKind {
	return e.kind
}

type SliceableDiff struct {
	valueDiff

	edits starlark.Tuple
}

func (d *SliceableDiff) String() string {
	var b strings.Builder
	b.WriteRune('[')
	for i, e := range d.edits {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(e.String())
	}
	b.WriteRune(']')
	return b.String()
}

func (d *SliceableDiff) Type() string {
	return "SliceableDiff"
}

func (d *SliceableDiff) Freeze() {
	d.freeze()
	d.edits.Freeze()
}

func (d *SliceableDiff) Attr(name string) (starlark.Value, error) {
	switch name {
	case "old":
		return d.old, nil
	case "new":
		return d.new, nil
	case "edits":
		return d.edits, nil
	default:
		return nil, nil
	}
}

func (d *SliceableDiff) AttrNames() []string {
	return []string{"old", "new", "edits"}
}

func (d *SliceableDiff) Edits() starlark.Sliceable {
	return d.edits
}

type SetDiff struct {
	valueDiff

	add    *starlark.Set
	remove *starlark.Set
}

func (d *SetDiff) String() string {
	return fmt.Sprintf("SetDiff(%v, %v)", d.Old(), d.New())
}

func (d *SetDiff) Type() string {
	return "SetDiff"
}

func (d *SetDiff) Freeze() {
	d.freeze()
	d.add.Freeze()
	d.remove.Freeze()
}

func (d *SetDiff) Attr(name string) (starlark.Value, error) {
	switch name {
	case "old":
		return d.old, nil
	case "new":
		return d.new, nil
	case "add":
		return d.add, nil
	case "remove":
		return d.remove, nil
	default:
		return nil, nil
	}
}

func (d *SetDiff) AttrNames() []string {
	return []string{"old", "new", "add", "remove"}
}

func (d *SetDiff) Add() *starlark.Set {
	return d.add
}

func (d *SetDiff) Remove() *starlark.Set {
	return d.remove
}
