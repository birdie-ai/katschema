package katschema

import (
	"encoding"
	"fmt"
)

// The AST **must**:
// - be pleasant to use (by hand)
// - be unambiguous (AST <-> TEXT)
// - be extensible
// - be easy to traverse.

// Design decisions:
// We are planning to use Katschema in replacing of JSON in places that does millions of
// data decoding per second, in both read and write endpoints (searchdsl, dml, dql, etc),
// because its properties allow for easier data traversing, validation and interpretation,
// as each node comes with its associated type (primitive or complex).
// The downside is that a naive AST would allocate a lot more than a conventional JSON decoder
// as each node would also hold a complex data structure for its type, even worse because they
// would be tiny duplicated pointers (as commonly same types are reused) which hurts GC scanning
// phase. Because all types and values have the same lifespan, an arena implementation is ideal
// but it conflicts with the possibility of a stateless AST.

type (
	// Type is a Katschema type.
	Type interface {
		encoding.TextMarshaler
		encoding.TextAppender

		fmt.Stringer
	}

	// TypeID is the type tag that lives in the Value AST.
	TypeID uint32

	// Value is a typed value.
	Value struct {
		typeid TypeID
		Value  any

		arena *Arena // optional, use Default if nil.
	}

	Ref string

	Schema struct {
		Ref  Ref
		Impl TypeID
		// TODO(i4k): constraints
	}

	Struct struct {
		Fields Fields

		arena *Arena
	}

	Field struct {
		Key string
		Typ TypeID
	}

	Fields []Field

	Tuple struct {
		ks    *Arena
		items []TypeID
	}

	List struct {
		ks    *Arena
		items TypeID
	}
)

func init() {
	Default = NewArena()

	Null = Default.Null
	Bool = Default.Bool
	Int = Default.Int
	Float = Default.Float
	String = Default.String
	True = Default.True
	False = Default.False
}

var (
	Default *Arena

	Null   TypeID
	Bool   TypeID
	Int    TypeID
	Float  TypeID
	String TypeID
	True   Value
	False  Value
)

func NewType(typ Type) TypeID { return Default.NewType(typ) }

func NewList(item TypeID) TypeID { return Default.List(item) }

func NewObj(fields ...Field) TypeID {
	return Default.Obj(fields...)
}

func NewTuple(fields ...TypeID) TypeID {
	return Default.Tuple(fields...)
}

func (r Ref) String() string { b, _ := r.MarshalText(); return string(b) }

func (s Schema) String() string { b, _ := s.MarshalText(); return string(b) }

func (l List) String() string { b, _ := l.MarshalText(); return string(b) }

func (t Tuple) String() string { b, _ := t.MarshalText(); return string(b) }

func (o Struct) String() string { b, _ := o.MarshalText(); return string(b) }

func NewValue(t TypeID, v any) Value {
	return Default.New(t, v)
}

func (v Value) Type() Type {
	arena := v.arena
	if v.arena == nil {
		arena = Default
	}
	return arena.Type(v.typeid)
}
