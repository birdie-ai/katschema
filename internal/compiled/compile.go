package compiled

import (
	"fmt"
	"strconv"

	"github.com/birdie-ai/katschema/parser/ast"
	"github.com/birdie-ai/katschema/parser/token"
)

type CompileError struct {
	Span token.Span
	Msg  string
}

func (e *CompileError) Error() string { return "compile: " + e.Msg }

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
		return 0, c.error(root, "optional can only be used in object fields")
	}
	return id, nil
}

type compiler struct {
	a *Arena
	t *ast.Tree
}

func (c *compiler) error(id ast.NodeID, format string, args ...any) error {
	return &CompileError{Span: c.t.Node(id).Span(), Msg: fmt.Sprintf(format, args...)}
}

func (c *compiler) value(id ast.NodeID, field bool) (TypeID, bool, error) {
	n := c.t.Node(id)
	switch n.Kind() {
	case ast.Null:
		return c.a.Null(), false, nil
	case ast.Bool:
		return c.a.internBool(c.t.Bool(id)), false, nil
	case ast.Int:
		t, err := c.intLiteral(id, c.t.Int(id))
		return t, false, err
	case ast.Float:
		t, err := c.floatLiteral(id, c.t.Float(id))
		return t, false, err
	case ast.String:
		return c.a.internStringLit(c.t.String(id)), false, nil
	case ast.List:
		t, err := c.array(id)
		return t, false, err
	case ast.Object:
		t, err := c.object(id)
		return t, false, err
	case ast.Schema:
		return c.schema(id, field)
	default:
		return 0, false, c.error(id, "%s is not a value or schema", n.Kind())
	}
}

func (c *compiler) intLiteral(id ast.NodeID, raw string) (TypeID, error) {
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, c.error(id, "integer literal %q is outside int64", raw)
	}
	return c.a.internInt(v), nil
}

func (c *compiler) floatLiteral(id ast.NodeID, raw string) (TypeID, error) {
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, c.error(id, "invalid number literal %q", raw)
	}
	return c.a.internFloat(v), nil
}

func (c *compiler) array(id ast.NodeID) (TypeID, error) {
	elems := c.t.List(id)
	if len(elems) == 1 && c.hasSchemaSyntax(elems[0]) {
		elem, optional, err := c.value(elems[0], false)
		if err != nil {
			return 0, err
		}
		if optional {
			return 0, c.error(elems[0], "optional is only valid on object fields")
		}
		return c.a.internList(elem), nil
	}
	for _, elem := range elems {
		if c.hasSchemaSyntax(elem) {
			return 0, c.error(id, "array schemas must contain exactly one element schema")
		}
	}

	out := make([]TypeID, len(elems))
	for i, elem := range elems {
		t, optional, err := c.value(elem, false)
		if err != nil {
			return 0, err
		}
		if optional {
			return 0, c.error(elem, "optional is only valid on object fields")
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
			return 0, c.error(id, "duplicate object field %q", name)
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
		fields[i] = Field{Name: c.a.internString(name), Type: t, Flags: flags}
	}
	return c.a.internObject(fields), nil
}

func (c *compiler) schema(id ast.NodeID, field bool) (TypeID, bool, error) {
	s := c.t.Schema(id)
	base, err := c.typeRef(s.Type)
	if err != nil {
		return 0, false, err
	}

	optional, err := c.optionality(s.Clauses)
	if err != nil {
		return 0, false, err
	}
	if optional && !field {
		return 0, false, c.error(id, "optional is only valid on object fields")
	}

	constraint, impossible, err := c.normalizeConstraints(base, s.Clauses)
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
		case "float":
			return c.a.Float(), nil
		case "string":
			return c.a.String(), nil
		default:
			return 0, c.error(id, "unresolved type %q", c.t.Name(id))
		}
	case ast.List:
		elems := c.t.List(id)
		if len(elems) != 1 {
			return 0, c.error(id, "array type must contain exactly one element schema")
		}
		elem, optional, err := c.value(elems[0], false)
		if err != nil {
			return 0, err
		}
		if optional {
			return 0, c.error(elems[0], "optional is only valid on object fields")
		}
		return c.a.internList(elem), nil
	case ast.Object:
		return c.object(id)
	case ast.Path:
		return 0, c.error(id, "type paths require a resolver")
	default:
		return 0, c.error(id, "%s cannot be used as a type reference", c.t.Node(id).Kind())
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
				return false, c.error(id, "optional must be boolean")
			}
			v = c.t.Bool(a.Value)
		}
		if set && val != v {
			return false, c.error(id, "conflicting optional clauses")
		}
		set, val = true, v
	}
	return set && val, nil
}
