package katschema

import (
	"cmp"
	"encoding"
)

type (
	// Type is a Katschema type.
	// The only constraint is that it should be marshaled as a valid katschema construct.
	Type interface {
		Value
	}

	Value interface {
		encoding.TextMarshaler
		encoding.TextAppender
	}

	// Typed value.
	Typed struct {
		Type  Schema
		Value Value
	}

	Ref string

	Schema struct {
		Type Type
		// TODO(i4k): constraints
	}

	// Null exist only to make katschema a superset of JSON, which means all JSON values are
	// valid Katschema values.
	Null struct{}

	Object[T cmp.Ordered] struct {
		Fields map[T]Type
	}

	List struct {
		Items Schema
	}

	Primitive struct {
		Value any
	}
)

var (
	Bool   = Schema{Type: Ref("bool")}
	Int    = Schema{Type: Ref("int")}
	Float  = Schema{Type: Ref("float")}
	String = Schema{Type: Ref("string")}

	True  = Typed{Type: Bool, Value: Primitive{Value: true}}
	False = Typed{Type: Bool, Value: Primitive{Value: false}}

	_ []Type = []Type{
		Ref(""),
		Null{},
		Object[string]{},
		Schema{},
		Typed{},
	}
)

func New(t Schema, v any) Typed {
	return Typed{
		Type:  t,
		Value: Primitive{Value: v},
	}
}

func New2(t Schema, v Value) Typed {
	return Typed{
		Type:  t,
		Value: v,
	}
}
