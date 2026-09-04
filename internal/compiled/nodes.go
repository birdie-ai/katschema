package compiled

type (
	TypeID       int32
	StringID     int32
	ConstraintID int32
	Kind         uint8
)

// NOTE(i4k): DO NOT CHANGE the order of the kind consts below lightly because it affects
// the partial ordering of types. Check [Arena.compareAtom].
// It does not mean it cannot be changed but just that doing that will canonicalize
// constraints like enums (and possibly others) differently and this could be catastrophic
// if the compiled semantic AST is being saved/restored (not done now, maybe never, probably
// a dumb idea but who knows what the future brings to this project).

const (
	Invalid Kind = iota
	Any
	Never
	Null
	Bool
	Int
	Float
	String

	// Atom kinds are compact scalar payloads referenced by constraints, usually defined as
	// expressions in the source code.
	// One such source of atoms are literals because they are interned as refined types.
	BoolAtom
	IntAtom
	FloatAtom
	StringAtom
	List
	Tuple
	Object
	Sum
	Refined
	Real
	RealAtom
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
	case Float:
		return "number"
	case Real:
		return "real"
	case String:
		return "string"
	case BoolAtom:
		return "bool atom"
	case IntAtom:
		return "int atom"
	case FloatAtom:
		return "number atom"
	case RealAtom:
		return "real atom"
	case StringAtom:
		return "string atom"
	case List:
		return "list"
	case Tuple:
		return "tuple"
	case Object:
		return "object"
	case Sum:
		return "sum"
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

// Field represents a field of an object.
//
// Field is guaranteed to remain comparable, so Field values may be compared
// directly using == and !=.
type Field struct {
	Name  StringID
	Value TypeID
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
