package katschema

import (
	"bytes"
	"encoding"
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
	"strconv"
)

var (
	ErrValueType      = errors.New(`value has invalid type`)
	ErrTypeUnresolved = errors.New(`unresolved type`)
)

type Encoder struct {
	buf io.Writer
}

func NewEncoder(buf io.Writer) *Encoder {
	return &Encoder{buf: buf}
}

func (enc *Encoder) EncodeValue(v Value) error {
	return enc.encodeValue(v)
}

func (enc *Encoder) encodeValue(v Value) error {
	arena := v.arena
	if arena == nil {
		arena = Default
	}
	// All Go primitive correctly encode to Katschema values.
	// Any algebraic type needs to implement the encoding interfaces.

	if avbuf, ok := enc.buf.(interface{ AvailableBuffer() []byte }); ok {
		// NOTE(i4k): The AvailableBuffer() method is a recent (go > 1.18) addition to stdlib
		// that allows for combining [io.Writer] and Append-like functions that works on []byte.
		// Check below:
		// https://pkg.go.dev/bytes#Buffer.AvailableBuffer
		// https://pkg.go.dev/bufio#Writer.AvailableBuffer

		if marshaler, ok := v.Value.(encoding.TextAppender); ok {
			b := avbuf.AvailableBuffer()
			var err error
			b, err = marshaler.AppendText(b)
			if err != nil {
				return err
			}
			return enc.write(b)
		}
	} else if marshaler, ok := v.Value.(encoding.TextMarshaler); ok {
		b, err := marshaler.MarshalText()
		if err != nil {
			return err
		}
		return enc.write(b)
	}

	// NOTE(i4k): if it falls here, the Value is not managed by the user.
	typ := v.Type()
	switch tt := typ.(type) {
	default:
		panic(v.Type)
	case Schema:
		switch tt.Ref {
		case "null":
			err := enc.writeString(nullstr)
			if err != nil {
				return err
			}
		case "int", "float", "bool":
			err := enc.writeString(fmt.Sprint(v.Value))
			if err != nil {
				return err
			}
		case "string":
			str, ok := v.Value.(strhdr)
			if !ok {
				return fmt.Errorf(`%w: value %v is incompatible with type %s`, ErrValueType, v.Value, v.Type())
			}
			err := enc.writeString(strconv.Quote(arena.str(str)))
			if err != nil {
				return err
			}
		default:
			// custom type
			if tt.Impl == Null {
				return fmt.Errorf(`%w: type %q`, ErrTypeUnresolved, tt.Ref)
			}
			newval := arena.New(tt.Impl, v.Value)
			return enc.EncodeValue(newval)
		}
	case List:
		err := enc.writeString("[")
		if err != nil {
			return err
		}
		vv, ok := v.Value.([]Value)
		if !ok {
			var err error
			// needs to be converted to katschema AST values
			vv, err = toValues(v.Value, arena)
			if err != nil {
				return err
			}
		}
		for i, item := range vv {
			if i > 0 {
				err := enc.writeString(",")
				if err != nil {
					return err
				}
			}
			err = enc.EncodeValue(item)
			if err != nil {
				return err
			}
		}
		err = enc.writeString("]")
		if err != nil {
			return err
		}
	case Struct:
		err := enc.writeString("{")
		if err != nil {
			return err
		}
		if v.Value != nil {
			vv, err := toValues(v.Value, arena)
			if err != nil {
				return err
			}
			for i, vvv := range vv {
				if i > 0 {
					err := enc.writeString(",")
					if err != nil {
						return err
					}
				}
				tt, ok := vvv.Type().(Tuple)
				if !ok {
					panic(vvv.Type())
				}
				if len(tt.items) != 2 {
					panic("TODO")
				}
				tt0 := arena.Type(tt.items[0])
				if sc, ok := tt0.(Schema); !ok || sc.Ref != "string" {
					panic("TODO")
				}
				vals, err := toValues(vvv.Value, arena)
				if err != nil {
					return err
				}
				err = enc.EncodeValue(vals[0])
				if err != nil {
					return err
				}
				err = enc.writeString(":")
				if err != nil {
					return err
				}
				err = enc.EncodeValue(vals[1])
				if err != nil {
					return err
				}
			}
		}
		err = enc.writeString("}")
		if err != nil {
			return err
		}
	}
	return nil
}

func (enc *Encoder) encodeType(v Value) error {
	return nil
}

func (enc *Encoder) write(b []byte) error {
	_, err := enc.buf.Write(b)
	return err
}

func (enc *Encoder) writeString(s string) error { return enc.write([]byte(s)) }

func (v Ref) AppendText(b []byte) ([]byte, error) { return append(b, string(v)...), nil }
func (v Ref) MarshalText() ([]byte, error)        { return v.AppendText([]byte{}) }

func (v Schema) AppendText(b []byte) ([]byte, error) {
	b = append(b, '(')
	var err error
	b, err = v.Ref.AppendText(b)
	if err != nil {
		return nil, err
	}
	b = append(b, ')')
	return b, nil
}

func (v Schema) MarshalText() ([]byte, error) {
	return v.AppendText([]byte{})
}

func (t Tuple) AppendText(b []byte) ([]byte, error) {
	b = append(b, '[')
	for i, v := range t.items {
		if i > 0 {
			b = append(b, ',')
		}
		var err error
		b, err = t.ks.Type(v).AppendText(b)
		if err != nil {
			return nil, err
		}
	}
	b = append(b, ']')
	return b, nil
}

func (t Tuple) MarshalText() ([]byte, error) {
	return t.AppendText([]byte{})
}

func (v List) AppendText(b []byte) ([]byte, error) {
	b = append(b, '[')
	var err error
	b, err = v.ks.Type(v.items).AppendText(b)
	if err != nil {
		return nil, err
	}
	b = append(b, ']')
	return b, nil
}

func (v List) MarshalText() ([]byte, error) {
	return v.AppendText([]byte{})
}

func (o Struct) AppendText(b []byte) ([]byte, error) {
	buf := bytes.NewBuffer(b)
	write(buf, "{")
	for i, field := range o.Fields {
		if i > 0 {
			write(buf, ",")
		}
		switch kk := any(field.Key).(type) {
		case string:
			write(buf, quote(kk))
		case int:
			write(buf, strconv.Itoa(kk))
		default:
			panic("unimplemented")
		}
		write(buf, ":")
		typ := o.arena.Type(field.Typ)
		vtext, err := typ.MarshalText()
		if err != nil {
			return nil, err
		}
		write(buf, string(vtext))
	}
	write(buf, "}")
	return buf.Bytes(), nil
}

func (o Struct) MarshalText() ([]byte, error) {
	return o.AppendText([]byte{})
}

func toValue(a any, arena *Arena) (Value, error) {
	if v, ok := a.(Value); ok {
		return v, nil
	}
	typ, val, err := arena.alloc(a)
	if err != nil {
		return Value{}, err
	}
	return Value{
		typeid: typ,
		Value:  val,
		arena:  arena,
	}, nil

}

func toValues(a any, ks *Arena) ([]Value, error) {
	var err error
	switch vv := a.(type) {
	case []Value:
		return vv, nil
	case []bool:
		vals := make([]Value, len(vv))
		for i := range vv {
			vals[i], err = toValue(vv[i], ks)
			if err != nil {
				return nil, err
			}
		}
		return vals, nil
	case []int:
		vals := make([]Value, len(vv))
		for i := range vv {
			vals[i], err = toValue(vv[i], ks)
			if err != nil {
				return nil, err
			}
		}
		return vals, nil
	case []float64:
		vals := make([]Value, len(vv))
		for i := range vv {
			vals[i], err = toValue(vv[i], ks)
			if err != nil {
				return nil, err
			}
		}
		return vals, nil
	case []string:
		vals := make([]Value, len(vv))
		for i := range vv {
			vals[i], err = toValue(vv[i], ks)
			if err != nil {
				return nil, err
			}
		}
		return vals, nil
	case []any:
		vals := make([]Value, len(vv))
		for i := range vv {
			vals[i], err = toValue(vv[i], ks)
			if err != nil {
				return nil, err
			}
		}
		return vals, nil

	// below are helper map values converted for convenience when setting up
	// objects from Go maps. This shall never be reached when encoding an AST
	// created by the parser.
	case map[string]any:
		keys := slices.Sorted(maps.Keys(vv))
		vals := make([]Value, len(keys))
		for i, k := range keys {
			val := vv[k]
			valtyp, vv, err := ks.alloc(val)
			if err != nil {
				return nil, err
			}
			vals[i] = ks.New(ks.Tuple(ks.String, valtyp), []Value{
				ks.New(ks.String, k),
				ks.New(valtyp, vv),
			})
		}
		return vals, nil
	default:
		return nil, fmt.Errorf(`%w: value %v (%T) cannot be encoded`, ErrValueType, a, a)
	}
}

func write(w io.Writer, p string) {
	_, _ = w.Write([]byte(p))
}

func quote(s string) string {
	return strconv.Quote(s)
}

// NOTE(i4k): some optimizations to avoid tiny alloc/copy inside the encoders.
const (
	nullstr = "null"
)
