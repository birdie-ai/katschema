package compiled

import (
	"errors"
	"fmt"
	"math/big"
	"strconv"

	"github.com/birdie-ai/katschema/math/decimal"
	"github.com/birdie-ai/katschema/parser/ast"
	"github.com/birdie-ai/katschema/parser/token"
)

type CompileError struct {
	Span token.Span
	Err  error
}

var ErrOptionalUnexpected = errors.New("optional is only valid in the object field")

func (e *CompileError) Error() string { return fmt.Sprintf("compile: %s", e.Err.Error()) }

// Compile lowers root into a canonical semantic type in the arena.
func Compile(a *Arena, t *ast.Tree, root ast.NodeID) (TypeID, error) {
	a.init()
	c := compiler{a: a, t: t}
	id, optional, err := c.value(root, false)
	if err != nil {
		return 0, err
	}

	// TODO(i4k): we have to decide if we allow optional in any value.
	// The problem is that if whole schema is (string, optional)
	// how does you provide emptiness? does it make sense to accept
	// a literal `null` for such type? I don't like this because null is
	// a literal value in Katschema, not the absense.
	// So maybe we should allow parsing an empty string into a ks.Never and
	// then consider this as a valid emptiness? we can easily address this
	// in the future, blocking now such usage until we decide what to do!
	if optional {
		return 0, c.error(root, ErrOptionalUnexpected)
	}
	return id, nil
}

type compiler struct {
	a *Arena
	t *ast.Tree
}

func (c *compiler) errorf(id ast.NodeID, format string, args ...any) error {
	return &CompileError{Span: c.t.Node(id).Span(), Err: fmt.Errorf(format, args...)}
}

func (c *compiler) error(id ast.NodeID, err error) error {
	return &CompileError{Span: c.t.Node(id).Span(), Err: err}
}

func (c *compiler) value(id ast.NodeID, field bool) (TypeID, bool, error) {
	n := c.t.Node(id)
	switch n.Kind() {
	case ast.Null, ast.Bool, ast.Int, ast.Decimal, ast.String:
		t, err := c.scalarLiteral(id)
		return t, false, err
	case ast.List:
		t, err := c.list(id)
		return t, false, err
	case ast.Object:
		t, err := c.object(id)
		return t, false, err
	case ast.Sum:
		t, err := c.sum(id)
		return t, false, err
	case ast.Schema:
		return c.schema(id, field)
	default:
		return 0, false, c.errorf(id, "%s is not a value or schema", n.Kind())
	}
}

func (c *compiler) scalarLiteral(id ast.NodeID) (TypeID, error) {
	atom, err := c.scalarLiteralAtom(id)
	if err != nil {
		return 0, err
	}

	var base TypeID
	switch c.t.Node(id).Kind() {
	case ast.Null:
		base = c.a.Null()
	case ast.Bool:
		base = c.a.Bool()
	case ast.Int:
		base = c.a.Int()
	case ast.Decimal:
		base = c.a.Real()
	case ast.String:
		base = c.a.String()
	default:
		panic("not a scalar literal")
	}
	return c.a.internLiteral(base, atom), nil
}

func (c *compiler) scalarLiteralAtom(id ast.NodeID) (TypeID, error) {
	switch c.t.Node(id).Kind() {
	case ast.Null:
		// NOTE(i4k): Null is both the null type and its only literal value.
		return c.a.Null(), nil
	case ast.Bool:
		return c.a.internBool(c.t.Bool(id)), nil
	case ast.Int:
		return c.intAtom(id, c.t.Int(id))
	case ast.Decimal:
		return c.realAtom(id, c.t.Decimal(id))
	case ast.String:
		return c.a.internStringAtom(c.t.String(id)), nil
	default:
		return 0, c.errorf(id, "%s is not a scalar literal", c.t.Node(id).Kind())
	}
}

func (c *compiler) intAtom(id ast.NodeID, raw string) (TypeID, error) {
	if v, err := strconv.ParseInt(raw, 10, 64); err == nil {
		return c.a.internInt(v), nil
	}
	var v big.Int
	if _, ok := v.SetString(raw, 10); !ok {
		return 0, c.errorf(id, "invalid integer literal %q", raw)
	}
	// NOTE(i4k): v.Bytes() usage is important here! it gives a big-endian encoded big int.
	return c.a.internRawBigInt(v.Sign() < 0, v.Bytes()), nil
}

func (c *compiler) realAtom(id ast.NodeID, raw string) (TypeID, error) {
	input := []byte(raw)
	parts, err := decimal.Parse(input, input[:0])
	if err != nil {
		return 0, c.errorf(id, "invalid number literal %q", raw)
	}
	return c.a.internRawReal(parts.Neg, parts.Digits, parts.Exp), nil
}

func (c *compiler) list(id ast.NodeID) (TypeID, error) {
	elems := c.t.List(id)
	if len(elems) == 1 && c.hasSchemaSyntax(elems[0]) {
		elem, optional, err := c.value(elems[0], false)
		if err != nil {
			return 0, err
		}
		if optional {
			return 0, c.error(elems[0], ErrOptionalUnexpected)
		}
		return c.a.internList(elem), nil
	}
	for _, elem := range elems {
		if c.hasSchemaSyntax(elem) {
			return 0, c.errorf(id, "array schemas must contain exactly one element schema")
		}
	}

	out := make([]TypeID, len(elems))
	for i, elem := range elems {
		t, optional, err := c.value(elem, false)
		if err != nil {
			return 0, err
		}
		if optional {
			return 0, c.error(elem, ErrOptionalUnexpected)
		}
		out[i] = t
	}
	return c.a.internTuple(out), nil
}

func (c *compiler) hasSchemaSyntax(id ast.NodeID) bool {
	switch c.t.Node(id).Kind() {
	case ast.Schema:
		return true
	case ast.List:
		for _, x := range c.t.List(id) {
			if c.hasSchemaSyntax(x) {
				return true
			}
		}
	case ast.Object:
		for _, f := range c.t.Object(id) {
			if c.hasSchemaSyntax(f.Value) {
				return true
			}
		}
	}
	return false
}

func (c *compiler) object(id ast.NodeID) (TypeID, error) {
	src := c.t.Object(id)
	fields := make([]Field, len(src))
	seen := make(map[string]struct{}, len(src))
	for i, f := range src {
		name := c.t.Text(f.Name)
		if _, ok := seen[name]; ok {
			return 0, c.errorf(id, "duplicate object field %q", name)
		}
		seen[name] = struct{}{}

		t, optional, err := c.value(f.Value, true)
		if err != nil {
			return 0, err
		}
		var flags FieldFlags
		if optional {
			flags |= FieldOptional
		}
		fields[i] = Field{Name: c.a.internString(name), Value: t, Flags: flags}
	}
	return c.a.internObject(fields), nil
}

func (c *compiler) sum(id ast.NodeID) (TypeID, error) {
	s := c.t.Sum(id)

	var members [2]TypeID
	left, optional, err := c.value(s.Left, false)
	if err != nil {
		return 0, err
	}
	if optional {
		return 0, c.error(s.Left, ErrOptionalUnexpected)
	}
	members[0] = left
	right, optional, err := c.value(s.Right, false)
	if err != nil {
		return 0, err
	}
	if optional {
		return 0, c.error(s.Left, ErrOptionalUnexpected)
	}
	members[1] = right
	return c.a.internSum(members), nil
}

func (c *compiler) schema(id ast.NodeID, field bool) (TypeID, bool, error) {
	s := c.t.Schema(id)
	base, err := c.typeRef(s.Type)
	if err != nil {
		return 0, false, err
	}

	// NOTE(i4k): When you do (int8, x >= 0) it actually means (int, x >= 0, x <= 127) because
	// (int8) is an alias for (int, -128 <= x <= 127).
	// So here we need to pull their intrinsic range into the same normalizer of this schema
	// refinement.
	initial := normConstraint{}
	if n := c.a.Node(base); n.kind == Refined {
		r := c.a.refinements[n.data]
		base = r.base
		switch c.a.Node(r.base).kind {
		case Int:
			initial = c.a.intConstraintNorm(r.constraint)
		case Real:
			initial = c.a.realConstraintNorm(r.constraint)
		default:
			// TODO(i4k): finish this
			panic("still unsupported refining refinements other than (int) and (real)")
		}
	}

	optional, err := c.optionality(s.Clauses)
	if err != nil {
		return 0, false, err
	}
	if optional && !field {
		return 0, false, c.error(id, ErrOptionalUnexpected)
	}
	constraint, impossible, err := c.extendNormalizeConstraints(base, initial, s.Clauses)
	if err != nil {
		return 0, false, err
	}
	if impossible {
		return c.a.Never(), optional, nil
	}
	return c.a.internRefined(base, constraint), optional, nil
}

func (c *compiler) typeRef(id ast.NodeID) (TypeID, error) {
	switch c.t.Node(id).Kind() {
	case ast.Name:
		switch c.t.Name(id) {
		case "any":
			return c.a.Any(), nil
		case "never":
			return c.a.Never(), nil
		case "null":
			return c.a.Null(), nil
		case "bool":
			return c.a.Bool(), nil
		case "int":
			return c.a.Int(), nil
		case "real":
			return c.a.Real(), nil
		case "string":
			return c.a.String(), nil
		case "int8":
			return c.a.Int8(), nil
		case "int16":
			return c.a.Int16(), nil
		case "int32":
			return c.a.Int32(), nil
		case "int64":
			return c.a.Int64(), nil
		case "uint8":
			return c.a.Uint8(), nil
		case "uint16":
			return c.a.Uint16(), nil
		case "uint32":
			return c.a.Uint32(), nil
		case "uint64":
			return c.a.Uint64(), nil
		case "float32":
			return c.a.Float32(), nil
		case "float64", "float":
			return c.a.Float64(), nil
		default:
			return 0, c.errorf(id, "unresolved type %q", c.t.Name(id))
		}
	case ast.List:
		elems := c.t.List(id)
		if len(elems) != 1 {
			return 0, c.errorf(id, "array type must contain exactly one element schema")
		}
		elem, optional, err := c.value(elems[0], false)
		if err != nil {
			return 0, err
		}
		if optional {
			return 0, c.error(elems[0], ErrOptionalUnexpected)
		}
		return c.a.internList(elem), nil
	case ast.Object:
		return c.object(id)
	case ast.Path:
		return 0, c.errorf(id, "type paths require a resolver")
	default:
		return 0, c.errorf(id, "%s cannot be used as a type reference", c.t.Node(id).Kind())
	}
}

func (c *compiler) optionality(clauses []ast.NodeID) (bool, error) {
	var (
		set bool
		val bool
	)
	for _, id := range clauses {
		if c.t.Node(id).Kind() != ast.Attr {
			continue
		}
		a := c.t.Attr(id)
		if c.t.Text(a.Name) != "optional" {
			continue
		}
		v := true
		if a.HasValue {
			if c.t.Node(a.Value).Kind() != ast.Bool {
				return false, c.errorf(id, "optional must be boolean")
			}
			v = c.t.Bool(a.Value)
		}
		if set && val != v {
			return false, c.errorf(id, "conflicting optional clauses")
		}
		set, val = true, v
	}
	return set && val, nil
}
