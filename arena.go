package katschema

import (
	"sync"

	"github.com/cespare/xxhash"
)

type (
	Arena struct {
		types   []Type
		bytes   []byte
		strings map[uint64]strhdr

		Null   TypeID
		Bool   TypeID
		Int    TypeID
		Float  TypeID
		String TypeID
		True   Value
		False  Value

		mu sync.Mutex
	}

	Option func(*arenaOptions)

	arenaOptions struct {
		size struct {
			types   int
			bytes   int
			strings int
		}
	}

	strhdr struct {
		pos int
		len int
	}
)

const (
	DefaultArenaSizeTypes   = 32
	DefaultArenaSizeBytes   = 1024
	DefaultArenaSizeStrings = 32
)

func NewArena(opts ...Option) *Arena {
	arenaOpts := arenaOptions{}
	for _, opt := range opts {
		opt(&arenaOpts)
	}
	if arenaOpts.size.types == 0 {
		arenaOpts.size.types = DefaultArenaSizeTypes
	}
	if arenaOpts.size.bytes == 0 {
		arenaOpts.size.types = DefaultArenaSizeBytes
	}
	if arenaOpts.size.strings == 0 {
		arenaOpts.size.strings = DefaultArenaSizeStrings
	}
	a := &Arena{
		types:   make([]Type, 0, arenaOpts.size.types),
		bytes:   make([]byte, 0, arenaOpts.size.bytes),
		strings: make(map[uint64]strhdr, arenaOpts.size.strings),
	}
	a.Null = a.NewType(Schema{Ref: Ref("null")})
	a.Bool = a.NewType(Schema{Ref: Ref("bool")})
	a.Int = a.NewType(Schema{Ref: Ref("int")})
	a.Float = a.NewType(Schema{Ref: Ref("float")})
	a.String = a.NewType(Schema{Ref: Ref("string")})
	a.True = a.New(a.Bool, true)
	a.False = a.New(a.Bool, false)
	return a
}

func WithArenaSizeTypes(size int) Option {
	return func(opts *arenaOptions) {
		opts.size.types = size
	}
}

func WithArenaSizeBytes(size int) Option {
	return func(opts *arenaOptions) {
		opts.size.bytes = size
	}
}

func WithArenaSizeStrings(size int) Option {
	return func(opts *arenaOptions) {
		opts.size.strings = size
	}
}

func (a *Arena) alloc(v any) (TypeID, any, error) {
	switch vv := v.(type) {
	case bool:
		return a.Bool, vv, nil
	case int, int8, int16, int32, int64:
		return Int, v, nil
	case float32, float64:
		return a.Float, vv, nil
	case string:
		return a.String, a.allocString(vv), nil
	case nil:
		return a.Null, nil, nil
	case Value:
		return vv.typeid, vv.Value, nil
	default:
		panic("not yet")
	}
}

func (a *Arena) allocString(s string) strhdr {
	hash := xxhash.Sum64([]byte(s))
	if v, ok := a.strings[hash]; ok {
		return v
	}
	pos := len(a.bytes)
	a.bytes = append(a.bytes, s...)
	hdr := strhdr{
		pos: pos,
		len: len(s),
	}
	a.strings[hash] = hdr
	return hdr
}

func (a *Arena) str(hdr strhdr) string {
	return string(a.bytes[hdr.pos : hdr.pos+hdr.len])
}

func (a *Arena) NewType(typ Type) TypeID {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.types = append(a.types, typ)
	return TypeID(len(a.types) - 1)
}

func (a *Arena) Type(id TypeID) Type {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.types[id]
}

func (a *Arena) New(typ TypeID, v any) Value {
	switch typ {
	case a.String:
		str, ok := v.(string)
		if ok {
			return Value{
				arena:  a,
				typeid: typ,
				Value:  a.allocString(str),
			}
		}
	}
	return Value{
		arena:  a,
		typeid: typ,
		Value:  v,
	}
}

func (a *Arena) Obj(fields ...Field) TypeID {
	return a.NewType(Struct{
		Fields: fields,
		arena:  a,
	})
}

func (a *Arena) List(typ TypeID) TypeID {
	return a.NewType(List{
		items: typ,
		ks:    a,
	})
}
func (a *Arena) Tuple(items ...TypeID) TypeID {
	return a.NewType(Tuple{
		ks:    a,
		items: items,
	})
}
