package pickle

import (
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"

	"github.com/pgavlin/starlark-go/starlark"
)

type failure error

type writer struct {
	w io.Writer
}

func (w writer) write(b []byte) int {
	if _, err := w.w.Write(b); err != nil {
		panic(failure(err))
	}
	return len(b)
}

func (w writer) Write(b []byte) (int, error) {
	return w.write(b), nil
}

func (w writer) writeByte(b byte) {
	buf := [1]byte{b}
	w.write(buf[:])
}

func (w writer) WriteByte(b byte) error {
	w.writeByte(b)
	return nil
}

func (w writer) writeString(s string) int {
	return w.write([]byte(s))
}

func (w writer) WriteString(s string) (int, error) {
	return w.writeString(s), nil
}

// A Pickler can return ErrCannotPickle to indicate that it does not support pickling a particular
// value.
var ErrCannotPickle = errors.New("cannot pickle")

// Pickler may be implemented to provide support for pickling non-primitive values.
type Pickler interface {
	// Pickle is called to pickle a non-primitive value.
	Pickle(x starlark.Value) (module, name string, args starlark.Tuple, err error)
}

// A PicklerFunc is an implementation of Pickler that implements Pickle by
// calling itself.
type PicklerFunc func(x starlark.Value) (module, name string, args starlark.Tuple, err error)

func (f PicklerFunc) Pickle(x starlark.Value) (module, name string, args starlark.Tuple, err error) {
	return f(x)
}

// An Encoder encodes values to an underlying Writer.
type Encoder struct {
	w       writer
	memo    map[starlark.Value]int
	pickler Pickler
}

// NewEncoder creates a new Encoder that writes to the given reader and pickles
// non-primitive values using the given Pickler.
func NewEncoder(w io.Writer, pickler Pickler) *Encoder {
	return &Encoder{
		w:       writer{w},
		memo:    map[starlark.Value]int{},
		pickler: pickler,
	}
}

func (e *Encoder) memoized(x starlark.Value) (int, bool) {
	if reflect.TypeOf(x).Comparable() {
		id, ok := e.memo[x]
		return id, ok
	}
	return 0, false
}

func (e *Encoder) memoize(x starlark.Value) {
	if reflect.TypeOf(x).Comparable() {
		id := len(e.memo)
		e.memo[x] = id

		e.w.writeByte(opMEMOIZE)
	}
}

func (e *Encoder) encodeString(opShort, opLong byte, x string) {
	var b [5]byte

	l := len(x)
	if l < 256 {
		b[0], b[1] = opShort, byte(l)
		e.w.write(b[:2])
	} else {
		b[0], b[1], b[2], b[3], b[4] = opLong, byte(l), byte(l>>8), byte(l>>16), byte(l>>24)
		e.w.write(b[:5])
	}

	e.w.writeString(x)
}

func (e *Encoder) encode(x starlark.Value) {
	// If we've memoized this object, emit a BINGET.
	if id, ok := e.memoized(x); ok {
		var b [5]byte
		if id < 256 {
			b[0], b[1] = opBINGET, byte(id)
			e.w.write(b[:2])
		} else {
			b[0], b[1], b[2], b[3], b[4] = opLONG_BINGET, byte(id), byte(id>>8), byte(id>>16), byte(id>>24)
			e.w.write(b[:5])
		}
		return
	}

	switch x := x.(type) {
	case starlark.NoneType:
		e.w.writeByte(opNONE)

	case starlark.Bool:
		if x {
			e.w.writeByte(opNEWTRUE)
		} else {
			e.w.writeByte(opNEWFALSE)
		}

	case starlark.Int:
		i64, ok := x.Int64()
		if !ok || i64 < math.MinInt32 || i64 > math.MaxInt32 {
			e.w.writeByte(opINT)
			text, err := x.BigInt().MarshalText()
			if err != nil {
				panic(failure(err))
			}
			e.w.write(text)
			e.w.writeByte('\n')
			return
		}

		var b [5]byte
		switch {
		case i64 >= 0 && i64 < 1<<8:
			b[0], b[1] = opBININT1, byte(i64)
			e.w.write(b[:2])
		case i64 >= 0 && i64 < 1<<16:
			b[0], b[1], b[2] = opBININT2, byte(i64), byte(i64>>8)
			e.w.write(b[:3])
		default:
			b[0], b[1], b[2], b[3], b[4] = opBININT, byte(i64), byte(i64>>8), byte(i64>>16), byte(i64>>24)
			e.w.write(b[:5])
		}

	case starlark.Float:
		bits := math.Float64bits(float64(x))

		var b [9]byte
		b[0] = opBINFLOAT
		b[1], b[2], b[3], b[4], b[5], b[6], b[7], b[8] = byte(bits), byte(bits>>8), byte(bits>>16), byte(bits>>24), byte(bits>>32), byte(bits>>40), byte(bits>>48), byte(bits>>56)
		e.w.write(b[:])

	case starlark.String:
		e.encodeString(opSHORT_BINUNICODE, opBINUNICODE, string(x))

	case starlark.Bytes:
		e.encodeString(opSHORT_BINBYTES, opBINBYTES, string(x))

	case starlark.Tuple:
		// TODO: recursive tuples

		switch len(x) {
		case 0:
			e.w.writeByte(opEMPTY_TUPLE)
		case 1:
			e.encode(x[0])
			e.w.writeByte(opTUPLE1)
		case 2:
			e.encode(x[0])
			e.encode(x[1])
			e.w.writeByte(opTUPLE2)
		case 3:
			e.encode(x[0])
			e.encode(x[1])
			e.encode(x[2])
			e.w.writeByte(opTUPLE3)
		default:
			e.w.writeByte(opMARK)
			for _, elem := range x {
				e.encode(elem)
			}
			e.w.writeByte(opTUPLE)
		}
		e.memoize(x)

	case *starlark.Set:
		e.w.writeByte(opEMPTY_SET)
		e.memoize(x)

		elems, first := x.Elems(), true
		for len(elems) > 0 {
			batch := elems
			if len(batch) > 1000 {
				batch = batch[:1000]
			}
			elems = elems[len(batch):]

			if !first {
				e.encode(x)
			}
			first = false

			e.w.writeByte(opMARK)
			for _, elem := range batch {
				e.encode(elem)
			}
			e.w.writeByte(opADDITEMS)
		}

	default:
		e.encodeComplex(x)
	}
}

func (e *Encoder) encodeComplex(x starlark.Value) {
	if e.pickler != nil {
		module, name, args, err := e.pickler.Pickle(x)
		switch err {
		case nil:
			e.encodeString(opSHORT_BINUNICODE, opBINUNICODE, module)
			e.encodeString(opSHORT_BINUNICODE, opBINUNICODE, name)
			e.w.writeByte(opSTACK_GLOBAL)
			e.encode(args)
			e.w.writeByte(opNEWOBJ)

			e.memoize(x)
			return
		case ErrCannotPickle:
			// OK
		default:
			panic(failure(err))
		}
	}

	switch x := x.(type) {
	case starlark.IterableMapping:
		e.w.writeByte(opEMPTY_DICT)
		e.memoize(x)

		items, first := x.Items(), true
		for len(items) > 0 {
			batch := items
			if len(batch) > 1000 {
				batch = batch[:1000]
			}
			items = items[len(batch):]

			if !first {
				e.encode(x)
			}
			first = false

			e.w.writeByte(opMARK)
			for _, kvp := range batch {
				e.encode(kvp[0])
				e.encode(kvp[1])
			}
			e.w.writeByte(opSETITEMS)
		}

	case starlark.Sequence:
		e.w.writeByte(opEMPTY_LIST)
		e.memoize(x)

		it := x.Iterate()
		defer it.Done()

		var el starlark.Value
		switch len := x.Len(); len {
		case 0:
			// done
		case 1:
			it.Next(&el)
			e.encode(el)
			e.w.writeByte(opAPPEND)
		default:
			first := true
			for i := 0; i < len; {
				batch := len - i
				if batch > 1000 {
					batch = 1000
				}

				if !first {
					e.encode(x)
				}
				first = false

				e.w.writeByte(opMARK)
				for ; batch > 0; i, batch = i+1, batch-1 {
					it.Next(&el)
					e.encode(el)
				}
				e.w.writeByte(opAPPENDS)
			}
		}

	case starlark.HasAttrs:
		e.w.writeByte(opEMPTY_DICT)
		e.memoize(x)

		attrs, first := x.AttrNames(), true
		for len(attrs) > 0 {
			batch := attrs
			if len(batch) > 1000 {
				batch = batch[:1000]
			}
			attrs = attrs[len(batch):]

			if !first {
				e.encode(x)
			}
			first = false

			e.w.writeByte(opMARK)
			for _, attr := range batch {
				e.encode(starlark.String(attr))
				v, err := x.Attr(attr)
				if err != nil {
					panic(failure(err))
				}
				e.encode(v)
			}
			e.w.writeByte(opSETITEMS)
		}

	default:
		panic(failure(fmt.Errorf("cannot pickle value of type %T", x)))
	}
}

// Encode encodes the given value to the underlying Writer.
func (e *Encoder) Encode(x starlark.Value) (err error) {
	defer func() {
		if f, ok := recover().(failure); ok {
			err = error(f)
		}
	}()

	e.encode(x)
	e.w.writeByte(opSTOP)
	return nil
}
