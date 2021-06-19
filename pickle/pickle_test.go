package pickle

import (
	"bytes"
	"errors"
	"math"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.starlark.net/starlark"
)

func testRoundTrip(t *testing.T, x starlark.Value, pickler Pickler, unpickler Unpickler) {
	t.Run(x.String(), func(t *testing.T) {
		var buf bytes.Buffer
		err := NewEncoder(&buf, pickler).Encode(x)
		require.NoError(t, err)
		y, err := NewDecoder(&buf, unpickler).Decode()
		require.NoError(t, err)
		eq, err := starlark.Equal(x, y)
		assert.NoError(t, err)
		if !assert.True(t, eq) {
			t.Logf("%v != %v", x, y)
		}
	})
}

func TestNone(t *testing.T) {
	testRoundTrip(t, starlark.None, nil, nil)
}

func TestBool(t *testing.T) {
	testRoundTrip(t, starlark.True, nil, nil)
	testRoundTrip(t, starlark.False, nil, nil)
}

func TestInt(t *testing.T) {
	cases := []string{
		"0",
		"42",
		"70000",
		"2147483647",
		"2147483648",
		"9223372036854775807",
		"-42",
		"-70000",
		"-2147483648",
		"-9223372036854775808",
	}
	for _, text := range cases {
		var i big.Int
		err := i.UnmarshalText([]byte(text))
		require.NoError(t, err)

		testRoundTrip(t, starlark.MakeBigInt(&i), nil, nil)
	}
}

func TestFloat(t *testing.T) {
	cases := []float64{
		0,
		42,
		70000,
		2147483647,
		2147483648,
		9223372036854775807,
		-42,
		-70000,
		-2147483648,
		-9223372036854775808,
		math.Pi,
		math.Inf(-1),
		math.Inf(1),
	}
	for _, c := range cases {
		testRoundTrip(t, starlark.Float(c), nil, nil)
	}
}

func TestString(t *testing.T) {
	cases := []string{
		"",
		"hello, world!",
		"Hello, 世界",
	}
	for _, c := range cases {
		testRoundTrip(t, starlark.String(c), nil, nil)
	}
}

func TestBytes(t *testing.T) {
	cases := [][]byte{
		nil,
		{0},
		[]byte("hello, world!"),
		{128, 129, 130},
	}
	for _, c := range cases {
		testRoundTrip(t, starlark.Bytes(c), nil, nil)
	}
}

func TestList(t *testing.T) {
	cases := []*starlark.List{
		starlark.NewList([]starlark.Value{starlark.String("hello"), starlark.MakeInt(42), starlark.True}),
		starlark.NewList(nil),
		starlark.NewList([]starlark.Value{starlark.None}),
	}
	for _, c := range cases {
		testRoundTrip(t, c, nil, nil)
	}

	var b bytes.Buffer
	rec := starlark.NewList(nil)
	rec.Append(rec)
	err := NewEncoder(&b, nil).Encode(rec)
	assert.NoError(t, err)
	lv, err := NewDecoder(&b, nil).Decode()
	assert.NoError(t, err)
	l := lv.(*starlark.List)
	assert.Equal(t, l, l.Index(0))
}

func TestTuple(t *testing.T) {
	cases := []starlark.Tuple{
		(nil),
		{starlark.None},
		{starlark.String("hello"), starlark.MakeInt(42)},
		{starlark.String("hello"), starlark.MakeInt(42), starlark.True},
		{starlark.String("hello"), starlark.MakeInt(42), starlark.True, starlark.None},
	}
	for _, c := range cases {
		testRoundTrip(t, c, nil, nil)
	}
}

func TestDict(t *testing.T) {
	cases := [][]starlark.Tuple{
		{{starlark.String("hello"), starlark.String("world")}, {starlark.True, starlark.MakeInt(42)}},
		{{starlark.True, starlark.MakeInt(42)}, {starlark.String("hello"), starlark.String("world")}},
		{{starlark.Float(42.0), starlark.True}},
	}
	for _, c := range cases {
		dict := starlark.NewDict(0)
		for _, kvp := range c {
			dict.SetKey(kvp[0], kvp[1])
		}
		testRoundTrip(t, dict, nil, nil)
	}
}

func TestSet(t *testing.T) {
	cases := []starlark.Tuple{
		{starlark.String("hello"), starlark.MakeInt(42), starlark.True},
		{starlark.Float(42.0)},
	}
	for _, c := range cases {
		set := starlark.NewSet(0)
		for _, v := range c {
			set.Insert(v)
		}
		testRoundTrip(t, set, nil, nil)
	}
}

func TestNewobj(t *testing.T) {
	fn := starlark.NewBuiltin("builtin", func(_ *starlark.Thread, _ *starlark.Builtin, _ starlark.Tuple, _ []starlark.Tuple) (starlark.Value, error) {
		return starlark.None, nil
	})

	pickle := PicklerFunc(func(x starlark.Value) (module, name string, args starlark.Tuple, err error) {
		if fn, ok := x.(*starlark.Builtin); ok {
			return "__main__", fn.Name(), nil, nil
		}
		return "", "", nil, errors.New("value is not a function")
	})

	unpickle := UnpicklerFunc(func(module, name string, args starlark.Tuple) (starlark.Value, error) {
		if module == "__main__" && name == fn.Name() && len(args) == 0 {
			return fn, nil
		}
		return nil, errors.New("unexpected args")
	})

	testRoundTrip(t, fn, pickle, unpickle)
}
