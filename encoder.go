package katschema

import (
	"bytes"
	"fmt"
	"io"
	"maps"
	"slices"
	"strconv"
)

func (v Ref) AppendText(b []byte) ([]byte, error) { return append(b, string(v)...), nil }
func (v Ref) MarshalText() ([]byte, error)        { return v.AppendText([]byte{}) }

func (v Typed) AppendText(b []byte) ([]byte, error) { return v.Value.AppendText(b) }
func (v Typed) MarshalText() ([]byte, error)        { return v.AppendText([]byte{}) }

func (v Null) AppendText(b []byte) ([]byte, error) { return append(b, 'n', 'u', 'l', 'l'), nil }
func (v Null) MarshalText() ([]byte, error)        { return v.AppendText([]byte{}) }

func (v Schema) AppendText(b []byte) ([]byte, error) {
	b = append(b, '(')
	var err error
	b, err = v.Type.AppendText(b)
	if err != nil {
		return nil, err
	}
	b = append(b, ')')
	return b, nil
}

func (v Schema) MarshalText() ([]byte, error) {
	return v.AppendText([]byte{})
}

func (v List) AppendText(b []byte) ([]byte, error) {
	switch v.Items.Type {
	case String:
		return append(b, `[(string)]`...), nil
	case Int:
		return append(b, `[(int)]`...), nil
	default:
		panic("unreachable")
	}
}

func (v List) MarshalText() ([]byte, error) {
	return v.AppendText([]byte{})
}

func (o Object[T]) AppendText(b []byte) ([]byte, error) {
	buf := bytes.NewBuffer(b)
	keys := slices.Sorted(maps.Keys(o.Fields))
	write(buf, "{")
	for i, k := range keys {
		if i > 0 {
			write(buf, ",")
		}
		switch kk := any(k).(type) {
		case string:
			write(buf, quote(kk))
		case int:
			write(buf, quote(strconv.Itoa(kk)))
		default:
			panic("unimplemented")
		}
		write(buf, ":")
		vtext, err := o.Fields[k].MarshalText()
		if err != nil {
			return nil, err
		}
		write(buf, string(vtext))
	}
	write(buf, "}")
	return buf.Bytes(), nil
}

func (v Primitive) AppendText(b []byte) ([]byte, error) {
	switch v.Value.(type) {
	case string:
		return append(b, strconv.Quote(fmt.Sprint(v.Value))...), nil
	default:
		return append(b, fmt.Sprint(v.Value)...), nil
	}
}
func (v Primitive) MarshalText() ([]byte, error) { return v.AppendText([]byte{}) }

func (o Object[T]) MarshalText() ([]byte, error) {
	return o.AppendText([]byte{})
}

func write(w io.Writer, p string) {
	_, _ = w.Write([]byte(p))
}

func quote(s string) string {
	return strconv.Quote(s)
}
