package pickle

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"strings"

	"github.com/pgavlin/starlark-go/starlark"
)

type reader struct {
	r io.Reader
}

func (r reader) Read(b []byte) (int, error) {
	n, err := io.ReadFull(r.r, b)
	if err != nil {
		panic(failure(err))
	}
	return n, nil
}

type markT int

func (markT) String() string        { return "mark" }
func (markT) Type() string          { return "mark" }
func (markT) Freeze()               {} // immutable
func (markT) Truth() starlark.Bool  { return starlark.False }
func (markT) Hash() (uint32, error) { return 0, nil }

var mark = markT(0)

type global struct {
	module, name string
}

func (*global) String() string        { return "global" }
func (*global) Type() string          { return "global" }
func (*global) Freeze()               {} // immutable
func (*global) Truth() starlark.Bool  { return starlark.False }
func (*global) Hash() (uint32, error) { return 0, nil }

// Unpickler may be implemented to provide support for unpickling non-primitive values.
type Unpickler interface {
	// Unpickle is called to unpickle a non-primitive value.
	Unpickle(module, name string, args starlark.Tuple) (starlark.Value, error)
}

// An UnpicklerFunc is an implementation of Unpickler that implements Unpickle by
// calling itself.
type UnpicklerFunc func(module, name string, args starlark.Tuple) (starlark.Value, error)

func (f UnpicklerFunc) Unpickle(module, name string, args starlark.Tuple) (starlark.Value, error) {
	return f(module, name, args)
}

// A Decoder decodes pickled values from an underlying Reader.
type Decoder struct {
	r         reader
	memo      []starlark.Value
	stack     []starlark.Value
	unpickler Unpickler
}

// NewDecoder creates a new Decoder that reads from the given reader and unpickles
// non-primitive values using the given Unpickler.
func NewDecoder(r io.Reader, unpickler Unpickler) *Decoder {
	return &Decoder{
		r:         reader{r},
		unpickler: unpickler,
	}
}

func (d *Decoder) push(x starlark.Value) {
	d.stack = append(d.stack, x)
}

func (d *Decoder) peek() starlark.Value {
	if len(d.stack) == 0 {
		panic(failure(errors.New("stack underflow")))
	}
	return d.stack[len(d.stack)-1]
}

func (d *Decoder) pop() starlark.Value {
	v := d.peek()
	d.stack = d.stack[:len(d.stack)-1]
	return v
}

func (d *Decoder) memoize(x starlark.Value) {
	d.memo = append(d.memo, x)
}

func (d *Decoder) get(id int) starlark.Value {
	if id >= len(d.memo) {
		panic(failure(fmt.Errorf("invalid object ID %v", id)))
	}
	return d.memo[id]
}

func (d *Decoder) readByte() byte {
	var buf [1]byte
	d.r.Read(buf[:])
	return buf[0]
}

func (d *Decoder) readUint32() uint32 {
	var b [4]byte
	d.r.Read(b[:])
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}

func (d *Decoder) readUint64() uint64 {
	var b [8]byte
	d.r.Read(b[:])
	return uint64(b[0]) | uint64(b[1])<<8 | uint64(b[2])<<16 | uint64(b[3])<<24 | uint64(b[4])<<32 | uint64(b[5])<<40 | uint64(b[6])<<48 | uint64(b[7])<<56
}

func (d *Decoder) decodeString(len int) string {
	var b strings.Builder
	b.Grow(len)

	if _, err := io.CopyN(&b, d.r, int64(len)); err != nil {
		panic(failure(err))
	}

	return b.String()
}

func (d *Decoder) decode() starlark.Value {
	for {
		switch op := d.readByte(); op {
		case opMARK:
			d.push(mark)
		case opMEMOIZE:
			d.memoize(d.peek())
		case opBINGET:
			d.push(d.get(int(d.readByte())))
		case opLONG_BINGET:
			d.push(d.get(int(d.readUint32())))
		case opSTOP:
			return d.pop()

		case opNONE:
			d.push(starlark.None)
		case opNEWTRUE:
			d.push(starlark.True)
		case opNEWFALSE:
			d.push(starlark.False)
		case opINT:
			var b bytes.Buffer
			for c := d.readByte(); c != '\n'; c = d.readByte() {
				b.WriteByte(c)
			}
			var i big.Int
			if err := i.UnmarshalText(b.Bytes()); err != nil {
				panic(failure(err))
			}
			d.push(starlark.MakeBigInt(&i))
		case opBININT1:
			d.push(starlark.MakeInt(int(d.readByte())))
		case opBININT2:
			l, h := d.readByte(), d.readByte()
			d.push(starlark.MakeInt(int(l) | int(h)<<16))
		case opBININT:
			d.push(starlark.MakeInt(int(int32(d.readUint32()))))
		case opBINFLOAT:
			d.push(starlark.Float(math.Float64frombits(d.readUint64())))
		case opSHORT_BINUNICODE:
			d.push(starlark.String(d.decodeString(int(d.readByte()))))
		case opBINUNICODE:
			d.push(starlark.String(d.decodeString(int(d.readUint32()))))
		case opSHORT_BINBYTES:
			d.push(starlark.Bytes(d.decodeString(int(d.readByte()))))
		case opBINBYTES:
			d.push(starlark.Bytes(d.decodeString(int(d.readUint32()))))
		case opEMPTY_LIST:
			d.push(starlark.NewList(nil))
		case opAPPEND:
			v := d.pop()
			l, ok := d.peek().(*starlark.List)
			if !ok {
				panic(failure(fmt.Errorf("APPEND expects a list, not a %s", d.peek().Type())))
			}
			l.Append(v)
		case opAPPENDS:
			if len(d.stack) == 0 {
				panic(failure(errors.New("stack underflow")))
			}
			i := len(d.stack) - 1
			for ; i > 0 && d.stack[i] != mark; i-- {
			}
			if i == 0 {
				panic(failure(errors.New("stack underflow")))
			}
			l, ok := d.stack[i-1].(*starlark.List)
			if !ok {
				panic(failure(fmt.Errorf("APPENDS expects a list, not a %s", d.stack[i-1].Type())))
			}
			for _, v := range d.stack[i+1:] {
				l.Append(v)
			}
			d.stack = d.stack[:i]
		case opEMPTY_TUPLE:
			d.push(starlark.Tuple{})
		case opTUPLE1:
			d.push(starlark.Tuple{d.pop()})
		case opTUPLE2:
			b, a := d.pop(), d.pop()
			d.push(starlark.Tuple{a, b})
		case opTUPLE3:
			c, b, a := d.pop(), d.pop(), d.pop()
			d.push(starlark.Tuple{a, b, c})
		case opTUPLE:
			if len(d.stack) == 0 {
				panic(failure(errors.New("stack underflow")))
			}
			i := len(d.stack) - 1
			for ; d.stack[i] != mark; i-- {
				if i == 0 {
					panic(failure(errors.New("stack underflow")))
				}
			}
			tuple := make(starlark.Tuple, len(d.stack)-i-1)
			copy(tuple, d.stack[i+1:])
			d.stack = d.stack[:i]
			d.push(tuple)
		case opEMPTY_DICT:
			d.push(starlark.NewDict(0))
		case opSETITEMS:
			if len(d.stack) == 0 {
				panic(failure(errors.New("stack underflow")))
			}
			i := len(d.stack) - 1
			for ; i > 0 && d.stack[i] != mark; i-- {
			}
			if i == 0 {
				panic(failure(errors.New("stack underflow")))
			}
			dict, ok := d.stack[i-1].(*starlark.Dict)
			if !ok {
				panic(failure(fmt.Errorf("SETITEMS expects a dict, not a %s", d.stack[i-1].Type())))
			}
			if (len(d.stack)-i-1)%2 != 0 {
				panic(failure(errors.New("SETITEMS expects an even number of values")))
			}
			for j := i + 1; j < len(d.stack); j += 2 {
				key, value := d.stack[j], d.stack[j+1]
				dict.SetKey(key, value)
			}
			d.stack = d.stack[:i]
		case opEMPTY_SET:
			d.push(starlark.NewSet(0))
		case opADDITEMS:
			if len(d.stack) == 0 {
				panic(failure(errors.New("stack underflow")))
			}
			i := len(d.stack) - 1
			for ; i > 0 && d.stack[i] != mark; i-- {
			}
			if i == 0 {
				panic(failure(errors.New("stack underflow")))
			}
			set, ok := d.stack[i-1].(*starlark.Set)
			if !ok {
				panic(failure(fmt.Errorf("ADDITEMS expects a set, not a %s", d.stack[i-1].Type())))
			}
			for _, v := range d.stack[i+1:] {
				set.Insert(v)
			}
			d.stack = d.stack[:i]
		case opSTACK_GLOBAL:
			nameV := d.pop()
			name, ok := nameV.(starlark.String)
			if !ok {
				panic(failure(fmt.Errorf("STACK_GLOBAL expects a string, not a %s", nameV.Type())))
			}

			moduleV := d.pop()
			module, ok := moduleV.(starlark.String)
			if !ok {
				panic(failure(fmt.Errorf("STACK_GLOBAL expects a string, not a %s", moduleV.Type())))
			}

			d.push(&global{module: string(module), name: string(name)})
		case opNEWOBJ:
			argsV := d.pop()
			args, ok := argsV.(starlark.Tuple)
			if !ok {
				panic(failure(fmt.Errorf("NEWOBJ expects a tuple, not a %s", argsV.Type())))
			}

			globalV := d.pop()
			global, ok := globalV.(*global)
			if !ok {
				panic(failure(fmt.Errorf("NEWOBJ expects a global, not a %s", globalV.Type())))
			}

			if d.unpickler == nil {
				panic(failure(errors.New("cannot decode NEWOBJ: no unpickler")))
			}

			v, err := d.unpickler.Unpickle(global.module, global.name, args)
			if err != nil {
				panic(failure(err))
			}
			d.push(v)
		default:
			panic(failure(fmt.Errorf("unimplemented opcode: 0x%02x", op)))
		}
	}
}

// Decode decodes the next value from the underlying Reader.
func (d *Decoder) Decode() (x starlark.Value, err error) {
	defer func() {
		if f, ok := recover().(failure); ok {
			err = error(f)
		}
	}()

	return d.decode(), nil
}
