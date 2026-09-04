package compiled

import (
	"math/big"
	"sort"
	"strconv"
	"unicode/utf8"

	"github.com/birdie-ai/katschema/math/decimal"
	"github.com/birdie-ai/katschema/parser/ast"
	"github.com/birdie-ai/katschema/parser/token"
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
	case Real:
		return astReal(t, value)
	case Float:
		return asFloat(t, value)
	case String:
		return t.Node(value).Kind() == ast.String
	case BoolAtom:
		return t.Node(value).Kind() == ast.Bool && t.Bool(value) == (n.data != 0)
	case IntAtom:
		if n.data > 0 {
			v, ok := astInt64(t, value)
			return ok && v == a.ints[n.data]
		}
		if t.Node(value).Kind() != ast.Int {
			return false
		}
		var v big.Int
		if _, ok := v.SetString(t.Int(value), 10); !ok {
			return false
		}
		return a.equalBigInts(n.data, v.Sign() < 0, v.Bytes())
	case FloatAtom:
		if t.Node(value).Kind() != ast.Decimal {
			return false
		}
		v, err := strconv.ParseFloat(t.Decimal(value), 64)
		return err == nil && floatEqual(v, a.floats[n.data])
	case RealAtom:
		got, ok := astRealNumber(t, value)
		if !ok {
			return false
		}
		want, ok := a.realNumber(typ)
		return ok && compareDecimalNumbers(got, want) == 0
	case StringAtom:
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
		if !a.valid(want[i].Value, t, f.Value) {
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
		if t.Node(value).Kind() != ast.Int || !a.rawIntWithinBounds(
			t.Int(value),
			intBounds{flags: d.intFlags, min: d.intMin, max: d.intMax},
		) {
			return false
		}
	}
	if d.flags&constraintReal != 0 {
		v, ok := astRealNumber(t, value)
		if !ok || !a.checkRealNumberBounds(v, d.realFlags, d.realMin, d.realMax) {
			return false
		}
	}
	if d.flags&constraintFloatConv != 0 {
		v, ok := astRealNumber(t, value)
		if !ok || !decimalCanBeFloat(v, d.format) {
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
	if t.Node(id).Kind() != ast.Int {
		return false
	}
	raw := t.Int(id)
	if _, err := strconv.ParseInt(raw, 10, 64); err == nil {
		return true
	}
	var v big.Int
	_, ok := v.SetString(raw, 10)
	return ok
}

func astReal(t *ast.Tree, id ast.NodeID) bool {
	_, ok := astRealNumber(t, id)
	return ok
}

func astRealNumber(t *ast.Tree, id ast.NodeID) (decimalNumber, bool) {
	switch t.Node(id).Kind() {
	case ast.Int:
		raw := t.Int(id)
		var v big.Int
		if _, ok := v.SetString(raw, 10); !ok {
			return decimalNumber{}, false
		}
		text := v.String()
		negative := len(text) > 0 && text[0] == '-'
		if negative {
			text = text[1:]
		}
		return decimalNumber{negative: negative, digits: []byte(text)}, true
	case ast.Decimal:
		parts, err := decimal.Parse([]byte(t.Decimal(id)), nil)
		if err != nil {
			return decimalNumber{}, false
		}
		return decimalNumber{negative: parts.Neg, digits: parts.Digits, exp: parts.Exp}, true
	case ast.Schema:
		s := t.Schema(id)
		lit, ok := literalValue(t, id)
		if !ok {
			return decimalNumber{}, false
		}
		lk := t.Node(lit).Kind()
		if lk != ast.Decimal {
			return decimalNumber{}, false
		}
		parts, err := decimal.Parse([]byte(t.Decimal(lit)), nil)
		if err != nil {
			return decimalNumber{}, false
		}
		dec := decimalNumber{negative: parts.Neg, digits: parts.Digits, exp: parts.Exp}
		switch t.Name(s.Type) {
		case "float32":
			if !decimalCanBeFloat(dec, f32Fmt) {
				return decimalNumber{}, false
			}
		case "float64":
			if !decimalCanBeFloat(dec, f64Fmt) {
				return decimalNumber{}, false
			}
		}
		return dec, true
	default:
		return decimalNumber{}, false
	}
}

func literalValue(t *ast.Tree, id ast.NodeID) (ast.NodeID, bool) {
	s := t.Schema(id)
	if len(s.Clauses) != 1 {
		return 0, false
	}
	constraint := t.Constraint(s.Clauses[0])
	node := t.Node(constraint)
	if node.Kind() != ast.Binary {
		return 0, false
	}
	b := t.Binary(constraint)
	if b.Op != token.Eq {
		return 0, false
	}
	if isX(t, b.Left) {
		return b.Right, true
	}
	return b.Left, true
}

func isX(t *ast.Tree, id ast.NodeID) bool {
	return t.Node(id).Kind() == ast.Ident && t.Ident(id) == "x"
}

func (a *Arena) checkRealNumberBounds(v decimalNumber, flags boundFlags, minID, maxID TypeID) bool {
	if flags&hasMin != 0 {
		min, ok := a.realNumber(minID)
		if !ok {
			return false
		}
		cmp := compareDecimalNumbers(v, min)
		if cmp < 0 || cmp == 0 && flags&minInclusive == 0 {
			return false
		}
	}
	if flags&hasMax != 0 {
		max, ok := a.realNumber(maxID)
		if !ok {
			return false
		}
		cmp := compareDecimalNumbers(v, max)
		if cmp > 0 || cmp == 0 && flags&maxInclusive == 0 {
			return false
		}
	}
	return true
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
		v, err := strconv.ParseFloat(t.Int(id), 64)
		return canonicalFloat(v), err == nil
	case ast.Decimal:
		v, err := strconv.ParseFloat(t.Decimal(id), 64)
		return canonicalFloat(v), err == nil
	default:
		return 0, false
	}
}
