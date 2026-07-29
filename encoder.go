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

func (v Ref) AppendText(b []byte) ([]byte, error) { return append(b, string(v)...), nil }
func (v Ref) MarshalText() ([]byte, error)        { return v.AppendText([]byte{}) }

func (v Value) AppendText(b []byte) ([]byte, error) {
	// All Go primitive correctly encode to Katschema values.
	// Any algebraic type needs to implement the encoding interfaces.

	if marshaler, ok := v.Value.(encoding.TextAppender); ok {
		return marshaler.AppendText(b)
	}
	if marshaler, ok := v.Value.(encoding.TextMarshaler); ok {
		data, err := marshaler.MarshalText()
		if err != nil {
			return nil, err
		}
		return append(b, data...), nil
	}

	switch tt := v.Type.(type) {
	default:
		panic(v.Type)
	case Schema:
		switch tt.Ref {
		case "null":
			b = append(b, 'n', 'u', 'l', 'l')
		case "int", "float", "bool":
			b = append(b, fmt.Sprint(v.Value)...)
		case "string":
			str, ok := v.Value.(string)
			if !ok {
				return nil, fmt.Errorf(`%w: value %v is incompatible with type %s`, ErrValueType, v.Value, v.Type)
			}
			b = append(b, strconv.Quote(str)...)
		default:
			// custom type
			if tt.Impl == nil {
				return nil, fmt.Errorf(`%w: type %q`, ErrTypeUnresolved, tt.Ref)
			}
			newval := Value{
				Type:  tt.Impl,
				Value: v.Value,
			}
			return newval.AppendText(b)
		}
	case List:
		b = append(b, '[')
		vv, ok := v.Value.([]Value)
		if !ok {
			var err error
			// needs to be converted to katschema AST values
			vv, err = toValues(v.Value)
			if err != nil {
				return nil, err
			}
		}
		for i, item := range vv {
			if i > 0 {
				b = append(b, ',')
			}
			vitem := Value{
				Type:  tt.Items,
				Value: item,
			}
			var err error
			b, err = vitem.AppendText(b)
			if err != nil {
				return nil, err
			}
		}
		b = append(b, ']')
	case Object[string]:
		b = append(b, '{')
		if v.Value != nil {
			vv, err := toValues(v.Value)
			if err != nil {
				panic(err)
			}
			for i, vvv := range vv {
				if i > 0 {
					b = append(b, ',')
				}
				tt, ok := vvv.Type.(Tuple)
				if !ok {
					panic("not yet")
				}
				if len(tt) != 2 {
					panic("TODO")
				}
				if sc, ok := tt[0].(Schema); !ok || sc.Ref != "string" {
					panic("TODO")
				}
				vals, err := toValues(vvv.Value)
				if err != nil {
					return nil, err
				}
				b, err = New(String, vals[0]).AppendText(b)
				if err != nil {
					return nil, err
				}
				b = append(b, ':')
				b, err = vals[1].AppendText(b)
				if err != nil {
					return nil, err
				}
			}
		}
		b = append(b, '}')
	case Object[int]:
		b = append(b, '{')
		if v.Value != nil {
			vv, ok := v.Value.([]Value)
			if !ok {
				panic("not yet")
			}
			for i, vvv := range vv {
				if i > 0 {
					b = append(b, ',')
				}
				tt, ok := vvv.Type.(Tuple)
				if !ok {
					panic("not yet")
				}
				if len(tt) != 2 {
					panic("TODO")
				}
				if sc, ok := tt[0].(Schema); !ok || sc.Ref != "int" {
					panic("unexpected")
				}
				vals, err := toValues(vvv.Value)
				if err != nil {
					return nil, err
				}
				b, err = New(Int, vals[0]).AppendText(b)
				if err != nil {
					return nil, err
				}
				b = append(b, ':')
				b, err = vals[1].AppendText(b)
				if err != nil {
					return nil, err
				}
			}
		}
		b = append(b, '}')
	}
	return b, nil
}

func (v Value) MarshalText() ([]byte, error) { return v.AppendText([]byte{}) }

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
	for i, v := range t {
		if i > 0 {
			b = append(b, ',')
		}
		var err error
		b, err = v.AppendText(b)
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
	b, err = v.Items.AppendText(b)
	if err != nil {
		return nil, err
	}
	b = append(b, ']')
	return b, nil
}

func (v List) MarshalText() ([]byte, error) {
	return v.AppendText([]byte{})
}

func (o Object[T]) AppendText(b []byte) ([]byte, error) {
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
		vtext, err := field.Typ.MarshalText()
		if err != nil {
			return nil, err
		}
		write(buf, string(vtext))
	}
	write(buf, "}")
	return buf.Bytes(), nil
}

func (o Object[T]) MarshalText() ([]byte, error) {
	return o.AppendText([]byte{})
}

func toValue(a any) (Value, error) {
	if v, ok := a.(Value); ok {
		return v, nil
	}
	typ, err := typeof(a)
	if err != nil {
		return Value{}, err
	}
	return Value{
		Type:  typ,
		Value: a,
	}, nil

}

func toValues(a any) ([]Value, error) {
	var err error
	switch vv := a.(type) {
	case []Value:
		return vv, nil
	case []bool:
		vals := make([]Value, len(vv))
		for i := range vv {
			vals[i], err = toValue(vv[i])
			if err != nil {
				return nil, err
			}
		}
		return vals, nil
	case []int:
		vals := make([]Value, len(vv))
		for i := range vv {
			vals[i], err = toValue(vv[i])
			if err != nil {
				return nil, err
			}
		}
		return vals, nil
	case []float64:
		vals := make([]Value, len(vv))
		for i := range vv {
			vals[i], err = toValue(vv[i])
			if err != nil {
				return nil, err
			}
		}
		return vals, nil
	case []string:
		vals := make([]Value, len(vv))
		for i := range vv {
			vals[i], err = toValue(vv[i])
			if err != nil {
				return nil, err
			}
		}
		return vals, nil
	case []any:
		vals := make([]Value, len(vv))
		for i := range vv {
			vals[i], err = toValue(vv[i])
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
			valtyp, err := typeof(val)
			if err != nil {
				return nil, err
			}
			vals[i] = New(Tuple{String, valtyp}, []Value{
				New(String, k),
				New(valtyp, val),
			})
		}
		return vals, nil
	case map[int]any:
		keys := slices.Sorted(maps.Keys(vv))
		vals := make([]Value, len(keys))
		for i, k := range keys {
			val := vv[k]
			valtyp, err := typeof(val)
			if err != nil {
				return nil, err
			}
			vals[i] = New(Tuple{Int, valtyp}, []Value{
				New(Int, k),
				New(valtyp, val),
			})
		}
		return vals, nil
	default:
		return nil, fmt.Errorf(`%w: value %v (%T) cannot be encoded`, ErrValueType, a, a)
	}
}

func typeof(a any) (Type, error) {
	switch v := a.(type) {
	case bool:
		return Bool, nil
	case int, int8, int16, int32, int64:
		return Int, nil
	case float32, float64:
		return Float, nil
	case string:
		return String, nil
	case nil:
		return Null, nil
	case Value:
		return v.Type, nil
	default:
		panic("not yet")
	}
}

func write(w io.Writer, p string) {
	_, _ = w.Write([]byte(p))
}

func quote(s string) string {
	return strconv.Quote(s)
}
