package compiled

import (
	"encoding/json"
	"fmt"
	"strconv"
	"unicode/utf8"
)

func (t Type) MarshalJSON() ([]byte, error) {
	if !t.isValue() {
		return nil, fmt.Errorf("katschema: cannot marshal non-literal type %s", t.Kind())
	}
	b, err := t.appendJSON(nil)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (t Type) appendJSON(dst []byte) ([]byte, error) {
	t = t.literalBase()
	n := t.a.Node(t.id)
	switch t.Kind() {
	case Null:
		return append(dst, "null"...), nil
	case BoolAtom:
		return strconv.AppendBool(dst, n.data != 0), nil
	case IntAtom:
		if n.data > 0 {
			return strconv.AppendInt(dst, t.a.int64(n.data), 10), nil
		}
		return t.a.bigInt(n.data).Append(dst, 10), nil
	case RealAtom:
		v, digits := t.a.realData(n.data)
		if v.negative() {
			dst = append(dst, '-')
		}
		dst = append(dst, digits...)
		dst = append(dst, 'e')
		return strconv.AppendInt(dst, v.exp, 10), nil
	case StringAtom:
		return appendJSONString(dst, t.a.StringValue(StringID(n.data)))
	case Tuple:
		dst = append(dst, '[')
		for i := 0; i < t.Len(); i++ {
			if i > 0 {
				dst = append(dst, ',')
			}
			var err error
			dst, err = t.At(i).appendJSON(dst)
			if err != nil {
				return nil, err
			}
		}
		return append(dst, ']'), nil
	case Object:
		dst = append(dst, '{')
		fields := t.Fields()
		for i := 0; i < fields.Len(); i++ {
			if i > 0 {
				dst = append(dst, ',')
			}
			field := fields.At(i)
			var err error
			dst, err = appendJSONString(dst, field.Name())
			if err != nil {
				return nil, err
			}
			dst = append(dst, ':')
			dst, err = field.Type().appendJSON(dst)
			if err != nil {
				return nil, err
			}
		}
		return append(dst, '}'), nil
	default:
		panic(t.Kind())
	}
}

func appendJSONString(dst []byte, s string) ([]byte, error) {
	if !utf8.ValidString(s) {
		return nil, fmt.Errorf("katschema: cannot marshal invalid UTF-8")
	}
	b, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	return append(dst, b...), nil
}
