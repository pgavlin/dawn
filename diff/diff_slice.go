package diff

import (
	"go.starlark.net/starlark"
)

// limit of cordinate size
const defaultRouteSize = 2000000

// point is coordinate in edit graph
type point struct {
	x, y int
}

// pointWithRoute is coordinate in edit graph attached route
type pointWithRoute struct {
	x, y, r int
}

type editKind int

const (
	editKindDelete editKind = iota
	editKindCommon
	editKindAdd
)

type edit struct {
	kind   editKind
	start  int
	values starlark.Sliceable
}

type differ struct {
	a, b    starlark.Sliceable
	m, n    int
	reverse bool
	depth   int

	ox, oy         int
	edits          []edit
	path           []int
	pointWithRoute []pointWithRoute
	routeSize      int
}

func diffSlice(a, b starlark.Sliceable, depth int) (*SliceableDiff, error) {
	m, n := a.Len(), b.Len()
	reverse := false
	if m >= n {
		a, b = b, a
		m, n = n, m
		reverse = true
	}

	d := differ{
		a:         a,
		b:         b,
		m:         m,
		n:         n,
		reverse:   reverse,
		depth:     depth,
		routeSize: defaultRouteSize,
	}
	edits, err := d.compose()
	if err != nil {
		return nil, err
	}
	return &SliceableDiff{
		valueDiff: valueDiff{old: a, new: b},
		edits:     edits,
	}, nil
}

var editKinds = []EditKind{EditKindDelete, EditKindCommon, EditKindAdd}

func slice(s starlark.Sliceable, start, len int) starlark.Sliceable {
	return s.Slice(start, len, 1).(starlark.Sliceable)
}

func indexReturnsSlice(v starlark.Sliceable) bool {
	switch v.(type) {
	case starlark.String, starlark.Bytes:
		return true
	default:
		return false
	}
}

func copySliceable(s starlark.Sliceable) starlark.Sliceable {
	if indexReturnsSlice(s) {
		return s
	}

	tuple := make(starlark.Tuple, s.Len())
	for i := range tuple {
		tuple[i] = s.Index(i)
	}
	return tuple
}

func diffReplacements(old, new starlark.Sliceable, depth int) (starlark.Tuple, error) {
	// Attempting to diff some elements will infinitely recur.
	if indexReturnsSlice(old) && indexReturnsSlice(new) {
		return starlark.Tuple{&LiteralDiff{valueDiff: valueDiff{old: old, new: new}}}, nil
	}

	diffs := make(starlark.Tuple, old.Len())
	for i := range diffs {
		d, err := DiffDepth(old.Index(i), new.Index(i), depth)
		if err != nil {
			return nil, err
		}
		if d == nil {
			diffs[i] = starlark.None
		} else {
			diffs[i] = d
		}
	}
	return diffs, nil
}

// compose composes diff between a and b
func (diff *differ) compose() (starlark.Tuple, error) {
	fp := make([]int, diff.m+diff.n+3)
	diff.path = make([]int, diff.m+diff.n+3)
	var epc []point

	for {
		diff.pointWithRoute = diff.pointWithRoute[:0]

		for i := range fp {
			fp[i] = -1
			diff.path[i] = -1
		}

		offset := diff.m + 1
		delta := diff.n - diff.m
		for p := 0; ; p++ {
			for k := -p; k <= delta-1; k++ {
				s, err := diff.snake(k, fp[k-1+offset]+1, fp[k+1+offset], offset)
				if err != nil {
					return nil, err
				}
				fp[k+offset] = s
			}

			for k := delta + p; k >= delta+1; k-- {
				s, err := diff.snake(k, fp[k-1+offset]+1, fp[k+1+offset], offset)
				if err != nil {
					return nil, err
				}
				fp[k+offset] = s
			}

			s, err := diff.snake(delta, fp[delta-1+offset]+1, fp[delta+1+offset], offset)
			if err != nil {
				return nil, err
			}
			fp[delta+offset] = s

			if fp[delta+offset] >= diff.n || len(diff.pointWithRoute) > diff.routeSize {
				break
			}
		}

		r := diff.path[delta+offset]
		epc := epc[:0]
		for r != -1 {
			epc = append(epc, point{x: diff.pointWithRoute[r].x, y: diff.pointWithRoute[r].y})
			r = diff.pointWithRoute[r].r
		}

		if diff.recordSeq(epc) {
			break
		}
	}

	edits := make(starlark.Tuple, 0, len(diff.edits))
	for _, e := range diff.edits {
		edit := &Edit{
			Sliceable: copySliceable(e.values),
			kind:      editKinds[int(e.kind)],
		}

		if len(edits) == 0 {
			edits = append(edits, edit)
			continue
		}

		// Can we merge this edit with the prior edit to create a replace?
		tail := edits[len(edits)-1].(*Edit)
		if e.kind != editKindAdd || tail.kind != EditKindDelete {
			edits = append(edits, edit)
			continue
		}

		old, new := tail.Sliceable, edit.Sliceable

		if old.Len() < new.Len() {
			// replace followed by add
			diffs, err := diffReplacements(old, slice(new, 0, old.Len()), diff.depth)
			if err != nil {
				return nil, err
			}
			tail.Sliceable, tail.kind = diffs, EditKindReplace

			edit.Sliceable = slice(new, old.Len(), new.Len())
			edits = append(edits, edit)
		} else if old.Len() > new.Len() {
			// replace followed by delete
			diffs, err := diffReplacements(slice(old, 0, new.Len()), new, diff.depth)
			if err != nil {
				return nil, err
			}
			tail.Sliceable, tail.kind = diffs, EditKindReplace

			edit.Sliceable, edit.kind = slice(old, new.Len(), old.Len()), EditKindDelete
			edits = append(edits, edit)
		} else {
			// pure replace
			diffs, err := diffReplacements(old, new, diff.depth)
			if err != nil {
				return nil, err
			}
			tail.Sliceable, tail.kind = diffs, EditKindReplace
		}
	}
	return edits, nil
}

func (diff *differ) snake(k, p, pp, offset int) (int, error) {
	r := 0
	if p > pp {
		r = diff.path[k-1+offset]
	} else {
		r = diff.path[k+1+offset]
	}

	y := max(p, pp)
	x := y - k

	for x < diff.m && y < diff.n {
		eq, err := starlark.EqualDepth(diff.a.Index(x), diff.b.Index(y), 1000)
		if err != nil {
			return 0, err
		}
		if !eq {
			break
		}

		x++
		y++
	}

	diff.path[k+offset] = len(diff.pointWithRoute)
	diff.pointWithRoute = append(diff.pointWithRoute, pointWithRoute{x: x, y: y, r: r})

	return y, nil
}

func (diff *differ) recordSeq(epc []point) bool {
	x, y := 1, 1
	px, py := 0, 0
	for i := len(epc) - 1; i >= 0; i-- {
		for (px < epc[i].x) || (py < epc[i].y) {
			if (epc[i].y - epc[i].x) > (py - px) {
				kind := editKindAdd
				if diff.reverse {
					kind = editKindDelete
				}
				diff.extend(kind, diff.b, py)

				y++
				py++
			} else if epc[i].y-epc[i].x < py-px {
				kind := editKindDelete
				if diff.reverse {
					kind = editKindAdd
				}
				diff.extend(kind, diff.a, px)

				x++
				px++
			} else {
				loc, from := px, diff.a
				if diff.reverse {
					loc, from = py, diff.b
				}
				diff.extend(editKindCommon, from, loc)

				x++
				y++
				px++
				py++
			}
		}
	}

	if x > diff.m && y > diff.n {
		// all recording succeeded
	} else {
		diff.a = slice(diff.a, x-1, diff.a.Len())
		diff.b = slice(diff.b, y-1, diff.b.Len())
		diff.m = diff.a.Len()
		diff.n = diff.b.Len()
		diff.ox = x - 1
		diff.oy = y - 1
		return false
	}

	return true
}

func (diff *differ) extend(kind editKind, from starlark.Sliceable, loc int) {
	if len(diff.edits) != 0 {
		last := &diff.edits[len(diff.edits)-1]
		if last.kind == kind && last.start+last.values.Len() == loc {
			last.values = slice(from, last.start, loc+1)
			return
		}
	}
	diff.edits = append(diff.edits, edit{kind: kind, start: loc, values: from.Slice(loc, loc+1, 1).(starlark.Sliceable)})
}

func max(x, y int) int {
	if x < y {
		return y
	}
	return x
}
