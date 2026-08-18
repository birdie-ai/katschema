package compiled

import (
	"sort"
	"strconv"
	"unicode/utf8"

	"github.com/birdie-ai/katschema/parser/ast"
)

// Valid reports whether the literal AST rooted at value is accepted by typ.
// The AST must contain data values, not schema expressions.
func (a *Arena) Valid(typ TypeID, values *ast.Tree, value ast.NodeID) bool {
	return a.valid(typ, values, value)
}

func (a *Arena) valid(typ TypeID, t *ast.Tree, value ast.NodeID) bool {
	n := a.Node(typ)
	switch n.kind {
	case Any:
		return t.Node(value).Kind().IsValue() && t.Node(value).Kind() != ast.Schema
	case Never:
		return false
	case Null:
		return t.Node(value).Kind() == ast.Null
	case Bool:
		return t.Node(value).Kind() == ast.Bool
	case Int:
		return astInt(t, value)
	case Float:
		return asFloat(t, value)
	case String:
		return t.Node(value).Kind() == ast.String
	case BoolLit:
		return t.Node(value).Kind() == ast.Bool && t.Bool(value) == (n.data != 0)
	case IntLit:
		v, ok := astInt64(t, value)
		return ok && v == a.ints[n.data]
	case FloatLit:
		if t.Node(value).Kind() != ast.Float {
			return false
		}
		v, err := strconv.ParseFloat(t.Float(value), 64)
		return err == nil && floatEqual(v, a.numbers[n.data])
	case StringLit:
		return t.Node(value).Kind() == ast.String && t.String(value) == a.StringValue(StringID(n.data))
	case List:
		if t.Node(value).Kind() != ast.List {
			return false
		}
		elem := TypeID(n.data)
		for _, v := range t.List(value) {
			if !a.valid(elem, t, v) {
				return false
			}
		}
		return true
	case Tuple:
		if t.Node(value).Kind() != ast.List {
			return false
		}
		want, got := a.tuple(typ), t.List(value)
		if len(want) != len(got) {
			return false
		}
		for i := range want {
			if !a.valid(want[i], t, got[i]) {
				return false
			}
		}
		return true
	case Object:
		return a.validObject(typ, t, value)
	case Refined:
		r := a.refinements[n.data]
		return a.valid(r.base, t, value) && a.validConstraint(r.constraint, t, value)
	}
	return false
}

func (a *Arena) validObject(typ TypeID, t *ast.Tree, value ast.NodeID) bool {
	if t.Node(value).Kind() != ast.Object {
		return false
	}
	want := a.objectFields(typ)
	got := t.Object(value)
	if len(got) > len(want) {
		return false
	}

	var small uint64
	var large []uint64
	if len(want) > 64 {
		large = make([]uint64, (len(want)+63)/64)
	}
	mark := func(i int) bool {
		if large == nil {
			bit := uint64(1) << uint(i)
			if small&bit != 0 {
				return false
			}
			small |= bit
			return true
		}
		word, bit := i/64, uint(i%64)
		mask := uint64(1) << bit
		if large[word]&mask != 0 {
			return false
		}
		large[word] |= mask
		return true
	}
	seen := func(i int) bool {
		if large == nil {
			return small&(uint64(1)<<uint(i)) != 0
		}
		return large[i/64]&(uint64(1)<<uint(i%64)) != 0
	}

	for _, f := range got {
		name := t.Text(f.Name)
		i := sort.Search(len(want), func(i int) bool {
			return a.StringValue(want[i].Name) >= name
		})
		if i == len(want) || a.StringValue(want[i].Name) != name || !mark(i) {
			return false
		}
		if !a.valid(want[i].Type, t, f.Value) {
			return false
		}
	}
	for i, f := range want {
		if !f.Optional() && !seen(i) {
			return false
		}
	}
	return true
}

func (a *Arena) validConstraint(id ConstraintID, t *ast.Tree, value ast.NodeID) bool {
	if id == 0 {
		return true
	}
	d := a.constraints[id]
	if d.flags&constraintInt != 0 {
		v, ok := astInt64(t, value)
		if !ok || !checkIntBounds(v, d.intFlags, d.intMin, d.intMax) {
			return false
		}
	}
	if d.flags&constraintFloat != 0 {
		v, ok := astFloat64(t, value)
		if !ok || !checkFloatBounds(v, d.floatFlags, d.numMin, d.numMax) {
			return false
		}
	}
	if d.flags&constraintLen != 0 {
		var l int64
		switch t.Node(value).Kind() {
		case ast.String:
			l = int64(utf8.RuneCountInString(t.String(value)))
		case ast.List:
			l = int64(len(t.List(value)))
		case ast.Object:
			l = int64(len(t.Object(value)))
		default:
			return false
		}
		if !checkIntBounds(l, d.lenFlags, d.lenMin, d.lenMax) {
			return false
		}
	}
	if d.flags&constraintEnum != 0 {
		values := a.constraintEnums[d.enum.off : d.enum.off+d.enum.len]
		for _, lit := range values {
			if a.valid(lit, t, value) {
				return true
			}
		}
		return false
	}
	return true
}

func astInt(t *ast.Tree, id ast.NodeID) bool {
	_, ok := astInt64(t, id)
	return ok
}

func astInt64(t *ast.Tree, id ast.NodeID) (int64, bool) {
	var raw string
	switch t.Node(id).Kind() {
	case ast.Int:
		raw = t.Int(id)
	default:
		return 0, false
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	return v, err == nil
}

func asFloat(t *ast.Tree, id ast.NodeID) bool {
	_, ok := astFloat64(t, id)
	return ok
}

func astFloat64(t *ast.Tree, id ast.NodeID) (float64, bool) {
	switch t.Node(id).Kind() {
	case ast.Int:
		// NOTE(i4k): the maximum integer representable in a float64 without loss of precision is 2^53.
		// Here we are solving a subtle bug that happens whenever you interpret a JSON number value
		// as integer in the application side, because when you parse a JSON number with only
		// an integral part that's bigger than 2^53 you silently lose information.
		// Better safe than sorry! we reject those cases and then the user must explicitly use the
		// float number syntax in the source in order to explicitly tell that loss of precision is
		// fine.
		// OpenAPI spec has `integer` and `int64`: https://swagger.io/docs/specification/v3_0/data-models/data-types/#numbers
		// but you have to protect yourself and sometimes buggy software don't even allow you to
		// handle this properly: https://github.com/ogen-go/ogen/issues/1144

		i, err := strconv.ParseInt(t.Int(id), 10, 64)
		if err != nil {
			return 0, false
		}
		if i > int64(2<<53) {
			return 0, false
		}
		return float64(i), true
	case ast.Float:
		v, err := strconv.ParseFloat(t.Float(id), 64)
		return canonicalFloat(v), err == nil
	default:
		return 0, false
	}
}
