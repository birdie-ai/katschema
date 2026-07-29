package katschema

import (
	"cmp"
	"encoding"
	"fmt"
)

// The AST **must**:
// - be pleasant to use (by hand)
// - be unambiguous (AST <-> TEXT)
// - be extensible

type (
	// Type is a Katschema type.
	Type interface {
		Check(Value) error

		encoding.TextMarshaler
		encoding.TextAppender

		fmt.Stringer
	}

	// Value is a typed value.
	Value struct {
		Type  Type
		Value any
	}

	Ref string

	Schema struct {
		Ref  Ref
		Impl Type
		// TODO(i4k): constraints
	}

	Object[T cmp.Ordered] struct {
		Fields Fields[T]
	}

	Field[T cmp.Ordered] struct {
		Key T
		Typ Type
	}

	Fields[T cmp.Ordered] []Field[T]

	Tuple []Type

	List struct {
		Items Type
	}
)

var (
	Null   = Schema{Ref: Ref("null")}
	Bool   = Schema{Ref: Ref("bool")}
	Int    = Schema{Ref: Ref("int")}
	Float  = Schema{Ref: Ref("float")}
	String = Schema{Ref: Ref("string")}

	True  = New(Bool, true)
	False = New(Bool, false)
)

func (r Ref) String() string      { b, _ := r.MarshalText(); return string(b) }
func (r Ref) Check(_ Value) error { return nil }

func (s Schema) String() string      { b, _ := s.MarshalText(); return string(b) }
func (s Schema) Check(_ Value) error { return nil }

func (l List) String() string      { b, _ := l.MarshalText(); return string(b) }
func (l List) Check(_ Value) error { return nil }

func (t Tuple) String() string      { b, _ := t.MarshalText(); return string(b) }
func (t Tuple) Check(_ Value) error { return nil }

func (o Object[T]) String() string      { b, _ := o.MarshalText(); return string(b) }
func (o Object[T]) Check(_ Value) error { return nil }

func New(t Type, v any) Value {
	return Value{
		Type:  t,
		Value: v,
	}
}
