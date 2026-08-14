// Package compiled contains Katschema's canonical semantic representation.
//
// An Arena is append-only while schemas are compiled. Type references are
// compact int32 IDs. Equal semantic types are interned and therefore share a
// TypeID within an arena.
package compiled

type TypeID int32
type StringID int32
type ConstraintID int32

type Kind uint8

const (
	Invalid Kind = iota
	Any
	Never
	Null
	Bool
	Int
	Number
	String
	BoolLit
	IntLit
	NumberLit
	StringLit
	List
	Tuple
	Object
	Refined
)

func (k Kind) String() string {
	switch k {
	case Any:
		return "any"
	case Never:
		return "never"
	case Null:
		return "null"
	case Bool:
		return "bool"
	case Int:
		return "int"
	case Number:
		return "number"
	case String:
		return "string"
	case BoolLit:
		return "bool literal"
	case IntLit:
		return "int literal"
	case NumberLit:
		return "number literal"
	case StringLit:
		return "string literal"
	case List:
		return "list"
	case Tuple:
		return "tuple"
	case Object:
		return "object"
	case Refined:
		return "refined"
	}
	return "invalid"
}

type Node struct {
	kind Kind
	data int32
}

func (n Node) Kind() Kind { return n.kind }

type FieldFlags uint8

const FieldOptional FieldFlags = 1 << 0

type Field struct {
	Name  StringID
	Type  TypeID
	Flags FieldFlags
}

func (f Field) Optional() bool { return f.Flags&FieldOptional != 0 }

type range32 struct {
	off int32
	len int32
}

type refinement struct {
	base       TypeID
	constraint ConstraintID
}

type boundFlags uint8

const (
	hasMin boundFlags = 1 << iota
	minInclusive
	hasMax
	maxInclusive
)

type constraintData struct {
	flags uint8

	intFlags boundFlags
	intMin   int64
	intMax   int64

	numFlags boundFlags
	numMin   float64
	numMax   float64

	lenFlags boundFlags
	lenMin   int64
	lenMax   int64

	enum range32
}

const (
	constraintInt uint8 = 1 << iota
	constraintNumber
	constraintLen
	constraintEnum
)
