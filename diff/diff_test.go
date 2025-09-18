package diff

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/pgavlin/starlark-go/starlark"
)

type S = starlark.String

func I(i int) starlark.Value { return starlark.MakeInt(i) }

func T(values ...starlark.Value) starlark.Tuple { return starlark.Tuple(values) }

func D(pairs ...starlark.Tuple) starlark.Value {
	dict := starlark.NewDict(len(pairs))
	for _, p := range pairs {
		dict.SetKey(p[0], p[1])
	}
	return dict
}

func TestDiff(t *testing.T) {
	cases := []struct {
		a, b starlark.Value
		want string
	}{
		{
			a:    S("abc"),
			b:    S("abd"),
			want: `[="ab", ~(("c" -> "d"),)]`,
		},
		{
			a:    S("abcd"),
			b:    S("abef"),
			want: `[="ab", ~(("cd" -> "ef"),)]`,
		},
		{
			a:    S("abcd"),
			b:    S("efcd"),
			want: `[~(("ab" -> "ef"),), ="cd"]`,
		},
		{
			a:    T(I(1), I(2), I(3)),
			b:    T(I(1), I(3), I(4)),
			want: `[=(1,), -(2,), =(3,), +(4,)]`,
		},
		{
			a:    T(T(I(1), I(2)), I(3)),
			b:    T(T(I(1), I(4)), I(3)),
			want: `[~([=(1,), ~((2 -> 4),)],), =(3,)]`,
		},
		{
			a:    D(T(S("foo"), S("bar")), T(I(42), I(24)), T(S("baz"), S("qux"))),
			b:    D(T(S("foo"), S("baz")), T(I(42), I(24)), T(S("qux"), S("baz"))),
			want: `{{"foo": ~([="ba", ~(("r" -> "z"),)],), "baz": -("qux",), "qux": +("baz",)}}`,
		},
		{
			a:    I(42),
			b:    I(24),
			want: `(42 -> 24)`,
		},
		{
			a:    T(I(42), I(24)),
			b:    I(32),
			want: `((42, 24) -> 32)`,
		},
		{
			a:    T(I(42), I(24)),
			b:    S("ab"),
			want: `[~((42 -> "a"), (24 -> "b"))]`,
		},
	}
	for _, c := range cases {
		diff, err := Diff(c.a, c.b)
		require.NoError(t, err)
		assert.Equal(t, c.want, diff.String())
	}
}
