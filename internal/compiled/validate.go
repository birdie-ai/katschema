package compiled

import (
	"math/big"
	"strconv"
	"unicode/utf8"

	"github.com/birdie-ai/katschema/math/decimal"
	"github.com/birdie-ai/katschema/parser/ast"
)

// Valid reports whether the AST rooted at value is accepted by typ.
func (a *Arena) Valid(typ TypeID, values *ast.Tree, value ast.NodeID) bool {
	return a.Type(typ).Valid(values, value)
}

// Valid validates literal AST nodes.
func (typ Type) Valid(t *ast.Tree, value ast.NodeID) bool {
	return typ.validAST(t, value, false)
}

func (typ Type) validAST(t *ast.Tree, value ast.NodeID, inList bool) bool {
	valueKind := t.Node(value).Kind()
	switch valueKind {
	case ast.Schema:
		if inList {
			return false
		}
		fallthrough
	case ast.Sum:
		values := NewArena()
		id, err := Compile(values, t, value)
		return err == nil && typ.ValidType(values.Type(id))
	}
	a := typ.a
	n := a.Node(typ.id)
	kind := n.kind
	if typ.view != nil {
		kind = typ.view.kind
	}
	switch kind {
	case Any:
		return typ.validAny(t, value, inList)
	case Invalid, Never:
		return false
	case Null:
		return valueKind == ast.Null
	case Bool:
		return valueKind == ast.Bool
	case Int:
		return astInt(t, value)
	case Real:
		return astReal(t, value)
	case String:
		return valueKind == ast.String
	case BoolAtom:
		return valueKind == ast.Bool && t.Bool(value) == (n.data != 0)
	case IntAtom:
		if n.data > 0 {
			v, ok := astInt64(t, value)
			return ok && v == a.ints[n.data]
		}
		if valueKind != ast.Int {
			return false
		}
		var v big.Int
		if _, ok := v.SetString(t.Int(value), 10); !ok {
			return false
		}
		return a.equalBigInts(n.data, v.Sign() < 0, v.Bytes())
	case RealAtom:
		got, ok := astRealNumber(t, value)
		if !ok {
			return false
		}
		want, ok := a.realNumber(typ.id)
		return ok && compareDecimalNumbers(got, want) == 0
	case StringAtom:
		return valueKind == ast.String && t.String(value) == a.StringValue(StringID(n.data))
	case List:
		if valueKind != ast.List {
			return false
		}
		element := a.Type(TypeID(n.data))
		if typ.view != nil {
			element = typ.view.child
		}
		for _, v := range t.List(value) {
			if !element.validAST(t, v, true) {
				return false
			}
		}
		return true
	case Tuple:
		if valueKind != ast.List {
			return false
		}
		want, got := a.tuple(typ.id), t.List(value)
		if len(want) != len(got) {
			return false
		}
		for i := range got {
			if !a.Type(want[i]).validAST(t, got[i], true) {
				return false
			}
		}
		return true
	case Sum:
		for _, member := range a.sum(typ.id) {
			if a.Type(member).validAST(t, value, inList) {
				return true
			}
		}
		return false
	case Object:
		return valueKind == ast.Object && typ.validObject(t, value, inList)
	case Refined:
		r := a.refinements[n.data]
		base := a.Type(r.base)
		if typ.view != nil {
			base = typ.view.child
		}
		return base.validAST(t, value, inList) && a.validConstraint(r.constraint, t, value)
	default:
		panic(kind)
	}
}

func (typ Type) validAny(t *ast.Tree, value ast.NodeID, inList bool) bool {
	switch t.Node(value).Kind() {
	case ast.Null, ast.Bool, ast.String:
		return true
	case ast.Int:
		return astInt(t, value)
	case ast.Decimal:
		return astReal(t, value)
	case ast.List:
		for _, child := range t.List(value) {
			if !typ.validAST(t, child, true) {
				return false
			}
		}
		return true
	case ast.Object:
		fields := t.Object(value)
		seen := make(map[string]struct{}, len(fields))
		for _, field := range fields {
			name := t.Text(field.Name)
			if _, exists := seen[name]; exists {
				return false
			}
			seen[name] = struct{}{}
			if !typ.validAST(t, field.Value, inList) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func (typ Type) validObject(t *ast.Tree, value ast.NodeID, inList bool) bool {
	want, got := typ.Fields(), t.Object(value)
	seen := make(map[string]bool, len(got))
	for _, f := range got {
		name := t.Text(f.Name)
		if seen[name] {
			return false
		}
		field, ok := want.Get(name)
		if !ok || !field.Type().validAST(t, f.Value, inList) {
			return false
		}
		seen[name] = true
	}
	for i := 0; i < want.Len(); i++ {
		field := want.At(i)
		if !field.Optional() && !seen[field.Name()] {
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
			if a.Type(lit).validAST(t, value, false) {
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

func astRealNumber(t *ast.Tree, id ast.NodeID) (decimal.Number, bool) {
	switch t.Node(id).Kind() {
	case ast.Int:
		raw := t.Int(id)
		var v big.Int
		if _, ok := v.SetString(raw, 10); !ok {
			return decimal.Number{}, false
		}
		text := v.String()
		negative := text[0] == '-'
		if negative {
			text = text[1:]
		}
		return decimal.Number{Neg: negative, Digits: []byte(text)}, true
	case ast.Decimal:
		parts, err := decimal.Parse([]byte(t.Decimal(id)), nil)
		return parts, err == nil
	default:
		return decimal.Number{}, false
	}
}

func (a *Arena) checkRealNumberBounds(v decimal.Number, flags boundFlags, minID, maxID TypeID) bool {
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
	if t.Node(id).Kind() != ast.Int {
		return 0, false
	}
	v, err := strconv.ParseInt(t.Int(id), 10, 64)
	return v, err == nil
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
