package compiled

import (
	"math"
	"sort"
	"strconv"
	"unicode/utf8"

	"github.com/birdie-ai/katschema/parser/ast"
	"github.com/birdie-ai/katschema/parser/token"
)

type intBounds struct {
	flags boundFlags
	min   int64
	max   int64
}

type numberBounds struct {
	flags boundFlags
	min   float64
	max   float64
}

type lenBounds struct {
	flags boundFlags
	min   int64
	max   int64
}

type normConstraint struct {
	ints    intBounds
	numbers numberBounds
	length  lenBounds
	enum    []TypeID // nil means no enum restriction; empty non-nil means impossible.
}

func (c *compiler) normalizeConstraints(base TypeID, clauses []ast.NodeID) (ConstraintID, bool, error) {
	n := normalizer{c: c, base: base}
	for _, id := range clauses {
		if c.t.Node(id).Kind() != ast.Constraint {
			continue
		}
		if err := n.consume(c.t.Constraint(id)); err != nil {
			return 0, false, err
		}
	}
	return n.finish()
}

type normalizer struct {
	c    *compiler
	base TypeID
	n    normConstraint
}

func (n *normalizer) consume(id ast.NodeID) error {
	id = n.ungroup(id)
	node := n.c.t.Node(id)
	if node.Kind() != ast.Binary {
		return n.c.error(id, "unsupported constraint expression %s", node.Kind())
	}

	b := n.c.t.Binary(id)
	if b.Op == token.And {
		if err := n.consume(b.Left); err != nil {
			return err
		}
		return n.consume(b.Right)
	}

	if isOrderOp(b.Op) {
		values, ops := n.flattenOrder(id)
		if len(ops) > 1 {
			for i, op := range ops {
				if err := n.compare(values[i], op, values[i+1]); err != nil {
					return err
				}
			}
			return nil
		}
		return n.compare(b.Left, b.Op, b.Right)
	}

	switch b.Op {
	case token.Eq:
		return n.equal(b.Left, b.Right)
	case token.In:
		return n.inSet(b.Left, b.Right)
	}
	return n.c.error(id, "unsupported constraint operator %s", b.Op)
}

func (n *normalizer) ungroup(id ast.NodeID) ast.NodeID {
	for n.c.t.Node(id).Kind() == ast.Group {
		id = n.c.t.Group(id)
	}
	return id
}

func isOrderOp(op token.Kind) bool {
	switch op {
	case token.Lt, token.Le, token.Gt, token.Ge:
		return true
	}
	return false
}

func invertOrder(op token.Kind) token.Kind {
	switch op {
	case token.Lt:
		return token.Gt
	case token.Le:
		return token.Ge
	case token.Gt:
		return token.Lt
	case token.Ge:
		return token.Le
	}
	return op
}

func (n *normalizer) flattenOrder(id ast.NodeID) ([]ast.NodeID, []token.Kind) {
	id = n.ungroup(id)
	b := n.c.t.Binary(id)
	left := n.ungroup(b.Left)
	if n.c.t.Node(left).Kind() == ast.Binary {
		lb := n.c.t.Binary(left)
		if isOrderOp(lb.Op) {
			values, ops := n.flattenOrder(left)
			return append(values, b.Right), append(ops, b.Op)
		}
	}
	return []ast.NodeID{b.Left, b.Right}, []token.Kind{b.Op}
}

func (n *normalizer) compare(left ast.NodeID, op token.Kind, right ast.NodeID) error {
	left = n.ungroup(left)
	right = n.ungroup(right)

	if n.isX(left) {
		return n.valueBound(op, right)
	}
	if n.isX(right) {
		return n.valueBound(invertOrder(op), left)
	}
	if n.isLenX(left) {
		return n.lengthBound(op, right)
	}
	if n.isLenX(right) {
		return n.lengthBound(invertOrder(op), left)
	}
	return n.c.error(left, "constraint comparison must reference x or len(x)")
}

func (n *normalizer) equal(left, right ast.NodeID) error {
	left = n.ungroup(left)
	right = n.ungroup(right)
	if n.isX(left) {
		return n.addEquality(right)
	}
	if n.isX(right) {
		return n.addEquality(left)
	}
	return n.c.error(left, "equality constraint must compare x with a literal")
}

func (n *normalizer) inSet(left, right ast.NodeID) error {
	left = n.ungroup(left)
	right = n.ungroup(right)
	if !n.isX(left) {
		return n.c.error(left, "left side of in must be x")
	}
	if n.c.t.Node(right).Kind() != ast.List {
		return n.c.error(right, "right side of in must be a literal array")
	}

	values := n.c.t.Array(right)
	enum := make([]TypeID, 0, len(values))
	for _, v := range values {
		id, err := n.literal(v)
		if err != nil {
			return err
		}
		if !n.c.a.literalCompatible(n.base, id) {
			return n.c.error(v, "enum literal is not compatible with %s", n.c.a.Node(n.base).Kind())
		}
		enum = append(enum, id)
	}
	n.addEnum(enum)
	return nil
}

func (n *normalizer) addEquality(id ast.NodeID) error {
	lit, err := n.literal(id)
	if err != nil {
		return err
	}
	if !n.c.a.literalCompatible(n.base, lit) {
		return n.c.error(id, "literal is not compatible with %s", n.c.a.Node(n.base).Kind())
	}
	n.addEnum([]TypeID{lit})
	return nil
}

func (n *normalizer) literal(id ast.NodeID) (TypeID, error) {
	id = n.ungroup(id)
	switch n.c.t.Node(id).Kind() {
	case ast.Null, ast.Bool, ast.Int, ast.Float, ast.String:
		t, optional, err := n.c.value(id, false)
		if optional {
			return 0, n.c.error(id, "optional literal is invalid in a constraint")
		}
		return t, err
	default:
		return 0, n.c.error(id, "constraint requires a scalar literal")
	}
}

func (n *normalizer) isX(id ast.NodeID) bool {
	id = n.ungroup(id)
	return n.c.t.Node(id).Kind() == ast.Ident && n.c.t.Ident(id) == "x"
}

func (n *normalizer) isLenX(id ast.NodeID) bool {
	id = n.ungroup(id)
	if n.c.t.Node(id).Kind() != ast.Call {
		return false
	}
	call := n.c.t.Fncall(id)
	return n.c.t.Text(call.Func) == "len" && len(call.Args) == 1 && n.isX(call.Args[0])
}

func (n *normalizer) valueBound(op token.Kind, literal ast.NodeID) error {
	kind := n.c.a.baseKind(n.base)
	raw, ok := n.numericRaw(literal)
	if !ok {
		return n.c.error(literal, "numeric comparison requires a numeric literal")
	}

	switch kind {
	case Int:
		if !isIntegerLexeme(raw) {
			return n.c.error(literal, "int constraint requires an integer bound")
		}
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return n.c.error(literal, "integer bound %q is outside int64", raw)
		}
		n.applyInt(op, v)
		return nil
	case Number:
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return n.c.error(literal, "invalid numeric bound %q", raw)
		}
		n.applyNumber(op, canonicalFloat(v))
		return nil
	default:
		return n.c.error(literal, "numeric comparison is invalid for %s", kind)
	}
}

func (n *normalizer) lengthBound(op token.Kind, literal ast.NodeID) error {
	kind := n.c.a.baseKind(n.base)
	switch kind {
	case String, List, Tuple, Object:
	default:
		return n.c.error(literal, "len(x) is invalid for %s", kind)
	}
	raw, ok := n.numericRaw(literal)
	if !ok || !isIntegerLexeme(raw) {
		return n.c.error(literal, "length bound must be an integer")
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || v < 0 {
		return n.c.error(literal, "length bound must be a non-negative int64")
	}
	n.applyLen(op, v)
	return nil
}

func (n *normalizer) numericRaw(id ast.NodeID) (string, bool) {
	id = n.ungroup(id)
	switch n.c.t.Node(id).Kind() {
	case ast.Int:
		return n.c.t.Int(id), true
	case ast.Float:
		return n.c.t.Float(id), true
	}
	return "", false
}

func (n *normalizer) applyInt(op token.Kind, v int64) {
	r := &n.n.ints
	switch op {
	case token.Gt, token.Ge:
		inclusive := op == token.Ge
		if r.flags&hasMin == 0 || v > r.min || v == r.min && !inclusive && r.flags&minInclusive != 0 {
			r.min = v
			r.flags |= hasMin
			if inclusive {
				r.flags |= minInclusive
			} else {
				r.flags &^= minInclusive
			}
		}
	case token.Lt, token.Le:
		inclusive := op == token.Le
		if r.flags&hasMax == 0 || v < r.max || v == r.max && !inclusive && r.flags&maxInclusive != 0 {
			r.max = v
			r.flags |= hasMax
			if inclusive {
				r.flags |= maxInclusive
			} else {
				r.flags &^= maxInclusive
			}
		}
	}
}

func (n *normalizer) applyNumber(op token.Kind, v float64) {
	r := &n.n.numbers
	switch op {
	case token.Gt, token.Ge:
		inclusive := op == token.Ge
		if r.flags&hasMin == 0 || v > r.min || v == r.min && !inclusive && r.flags&minInclusive != 0 {
			r.min = v
			r.flags |= hasMin
			if inclusive {
				r.flags |= minInclusive
			} else {
				r.flags &^= minInclusive
			}
		}
	case token.Lt, token.Le:
		inclusive := op == token.Le
		if r.flags&hasMax == 0 || v < r.max || v == r.max && !inclusive && r.flags&maxInclusive != 0 {
			r.max = v
			r.flags |= hasMax
			if inclusive {
				r.flags |= maxInclusive
			} else {
				r.flags &^= maxInclusive
			}
		}
	}
}

func (n *normalizer) applyLen(op token.Kind, v int64) {
	r := &n.n.length
	switch op {
	case token.Gt, token.Ge:
		inclusive := op == token.Ge
		if r.flags&hasMin == 0 || v > r.min || v == r.min && !inclusive && r.flags&minInclusive != 0 {
			r.min = v
			r.flags |= hasMin
			if inclusive {
				r.flags |= minInclusive
			} else {
				r.flags &^= minInclusive
			}
		}
	case token.Lt, token.Le:
		inclusive := op == token.Le
		if r.flags&hasMax == 0 || v < r.max || v == r.max && !inclusive && r.flags&maxInclusive != 0 {
			r.max = v
			r.flags |= hasMax
			if inclusive {
				r.flags |= maxInclusive
			} else {
				r.flags &^= maxInclusive
			}
		}
	}
}

func (n *normalizer) addEnum(v []TypeID) {
	v = n.c.a.sortUniqueTypes(v)
	if n.n.enum == nil {
		n.n.enum = v
		return
	}
	out := make([]TypeID, 0, min(len(n.n.enum), len(v)))
	i, j := 0, 0
	for i < len(n.n.enum) && j < len(v) {
		a, b := n.n.enum[i], v[j]
		cmp := n.c.a.compareLiteral(a, b)
		switch {
		case cmp < 0:
			i++
		case cmp > 0:
			j++
		default:
			out = append(out, a)
			i++
			j++
		}
	}
	n.n.enum = out
}

func (n *normalizer) finish() (ConstraintID, bool, error) {
	if contradictoryInt(n.n.ints) || contradictoryNumber(n.n.numbers) || contradictoryLen(n.n.length) {
		return 0, true, nil
	}

	kind := n.c.a.baseKind(n.base)
	if n.n.enum == nil {
		switch kind {
		case Int:
			if singletonInt(n.n.ints) {
				n.n.enum = []TypeID{n.c.a.internInt(n.n.ints.min)}
				n.n.ints = intBounds{}
			}
		case Number:
			if singletonNumber(n.n.numbers) {
				n.n.enum = []TypeID{n.c.a.internNumber(n.n.numbers.min)}
				n.n.numbers = numberBounds{}
			}
		}
	}

	if n.n.enum != nil {
		out := n.n.enum[:0]
		for _, id := range n.n.enum {
			if n.c.a.literalSatisfiesNorm(id, n.n) {
				out = append(out, id)
			}
		}
		n.n.enum = out
		if len(out) == 0 {
			return 0, true, nil
		}
		// Once the finite set has been filtered, the other restrictions are
		// redundant and would only create multiple encodings for the same set.
		n.n.ints = intBounds{}
		n.n.numbers = numberBounds{}
		n.n.length = lenBounds{}
	}

	if n.n.ints.flags == 0 && n.n.numbers.flags == 0 && n.n.length.flags == 0 && n.n.enum == nil {
		return 0, false, nil
	}
	return n.c.a.internConstraint(n.n), false, nil
}

func contradictoryInt(r intBounds) bool {
	if r.flags&(hasMin|hasMax) != hasMin|hasMax {
		return false
	}
	if r.min > r.max {
		return true
	}
	return r.min == r.max && (r.flags&minInclusive == 0 || r.flags&maxInclusive == 0)
}

func contradictoryNumber(r numberBounds) bool {
	if r.flags&(hasMin|hasMax) != hasMin|hasMax {
		return false
	}
	if r.min > r.max {
		return true
	}
	return r.min == r.max && (r.flags&minInclusive == 0 || r.flags&maxInclusive == 0)
}

func contradictoryLen(r lenBounds) bool {
	if r.flags&(hasMin|hasMax) != hasMin|hasMax {
		return false
	}
	if r.min > r.max {
		return true
	}
	return r.min == r.max && (r.flags&minInclusive == 0 || r.flags&maxInclusive == 0)
}

func singletonInt(r intBounds) bool {
	return r.flags&(hasMin|minInclusive|hasMax|maxInclusive) == hasMin|minInclusive|hasMax|maxInclusive && r.min == r.max
}

func singletonNumber(r numberBounds) bool {
	return r.flags&(hasMin|minInclusive|hasMax|maxInclusive) == hasMin|minInclusive|hasMax|maxInclusive && r.min == r.max
}

func (a *Arena) internConstraint(n normConstraint) ConstraintID {
	a.scratch = a.scratch[:0]
	a.scratch = append(a.scratch, encodingVersion, 0xc1)
	if n.ints.flags != 0 {
		a.scratch = append(a.scratch, constraintInt, byte(n.ints.flags))
		a.scratch = appendInt64(a.scratch, n.ints.min)
		a.scratch = appendInt64(a.scratch, n.ints.max)
	}
	if n.numbers.flags != 0 {
		a.scratch = append(a.scratch, constraintNumber, byte(n.numbers.flags))
		a.scratch = appendFloat64(a.scratch, n.numbers.min)
		a.scratch = appendFloat64(a.scratch, n.numbers.max)
	}
	if n.length.flags != 0 {
		a.scratch = append(a.scratch, constraintLen, byte(n.length.flags))
		a.scratch = appendInt64(a.scratch, n.length.min)
		a.scratch = appendInt64(a.scratch, n.length.max)
	}
	if n.enum != nil {
		a.scratch = append(a.scratch, constraintEnum)
		a.scratch = appendInt32(a.scratch, int32(len(n.enum)))
		for _, id := range n.enum {
			a.scratch = appendUint64(a.scratch, a.Fingerprint(id))
		}
	}
	fp := a.hash(a.scratch)

	for id := a.constraintHead[fp]; id != 0; id = a.constraintNext[id] {
		if a.constraintEqual(id, n) {
			return id
		}
	}

	d := constraintData{}
	if n.ints.flags != 0 {
		d.flags |= constraintInt
		d.intFlags, d.intMin, d.intMax = n.ints.flags, n.ints.min, n.ints.max
	}
	if n.numbers.flags != 0 {
		d.flags |= constraintNumber
		d.numFlags, d.numMin, d.numMax = n.numbers.flags, n.numbers.min, n.numbers.max
	}
	if n.length.flags != 0 {
		d.flags |= constraintLen
		d.lenFlags, d.lenMin, d.lenMax = n.length.flags, n.length.min, n.length.max
	}
	if n.enum != nil {
		d.flags |= constraintEnum
		d.enum = range32{off: int32(len(a.constraintEnums)), len: int32(len(n.enum))}
		a.constraintEnums = append(a.constraintEnums, n.enum...)
	}

	id := ConstraintID(len(a.constraints))
	a.constraints = append(a.constraints, d)
	a.constraintHash = append(a.constraintHash, fp)
	a.constraintNext = append(a.constraintNext, a.constraintHead[fp])
	a.constraintHead[fp] = id
	return id
}

func (a *Arena) constraintEqual(id ConstraintID, n normConstraint) bool {
	d := a.constraints[id]
	if (d.flags&constraintInt != 0) != (n.ints.flags != 0) ||
		(d.flags&constraintNumber != 0) != (n.numbers.flags != 0) ||
		(d.flags&constraintLen != 0) != (n.length.flags != 0) ||
		(d.flags&constraintEnum != 0) != (n.enum != nil) {
		return false
	}
	if n.ints.flags != 0 && (d.intFlags != n.ints.flags || d.intMin != n.ints.min || d.intMax != n.ints.max) {
		return false
	}
	if n.numbers.flags != 0 && (d.numFlags != n.numbers.flags || !floatEqual(d.numMin, n.numbers.min) || !floatEqual(d.numMax, n.numbers.max)) {
		return false
	}
	if n.length.flags != 0 && (d.lenFlags != n.length.flags || d.lenMin != n.length.min || d.lenMax != n.length.max) {
		return false
	}
	if n.enum != nil {
		old := a.constraintEnums[d.enum.off : d.enum.off+d.enum.len]
		if !equalTypeIDs(old, n.enum) {
			return false
		}
	}
	return true
}

func (a *Arena) constraintFingerprint(id ConstraintID) uint64 {
	if id <= 0 || int(id) >= len(a.constraintHash) {
		return 0
	}
	return a.constraintHash[id]
}

func (a *Arena) sortUniqueTypes(v []TypeID) []TypeID {
	sort.Slice(v, func(i, j int) bool { return a.compareLiteral(v[i], v[j]) < 0 })
	out := v[:0]
	for _, id := range v {
		if len(out) == 0 || out[len(out)-1] != id {
			out = append(out, id)
		}
	}
	return out
}

// compareLiteral gives finite literal sets an order independent of arena IDs
// and fingerprints. This matters even when two XXH64 fingerprints collide.
func (a *Arena) compareLiteral(x, y TypeID) int {
	xn, yn := a.Node(x), a.Node(y)
	if xn.kind < yn.kind {
		return -1
	}
	if xn.kind > yn.kind {
		return 1
	}
	switch xn.kind {
	case Null:
		return 0
	case BoolLit:
		if xn.data < yn.data {
			return -1
		}
		if xn.data > yn.data {
			return 1
		}
	case IntLit:
		xv, yv := a.ints[xn.data], a.ints[yn.data]
		if xv < yv {
			return -1
		}
		if xv > yv {
			return 1
		}
	case NumberLit:
		xv, yv := a.numbers[xn.data], a.numbers[yn.data]
		if xv < yv {
			return -1
		}
		if xv > yv {
			return 1
		}
	case StringLit:
		xv, yv := a.StringValue(StringID(xn.data)), a.StringValue(StringID(yn.data))
		if xv < yv {
			return -1
		}
		if xv > yv {
			return 1
		}
	}
	return 0
}

func (a *Arena) baseKind(id TypeID) Kind {
	for a.Node(id).kind == Refined {
		id = a.refinements[a.Node(id).data].base
	}
	return a.Node(id).kind
}

func (a *Arena) literalCompatible(base, lit TypeID) bool {
	bk := a.baseKind(base)
	lk := a.Node(lit).kind
	switch bk {
	case Any:
		return true
	case Null:
		return lk == Null
	case Bool:
		return lk == BoolLit
	case Int:
		return lk == IntLit
	case Number:
		return lk == IntLit || lk == NumberLit
	case String:
		return lk == StringLit
	}
	return false
}

func (a *Arena) literalSatisfiesNorm(id TypeID, n normConstraint) bool {
	node := a.Node(id)
	if n.ints.flags != 0 {
		if node.kind != IntLit || !checkIntBounds(a.ints[node.data], n.ints.flags, n.ints.min, n.ints.max) {
			return false
		}
	}
	if n.numbers.flags != 0 {
		var v float64
		switch node.kind {
		case IntLit:
			v = float64(a.ints[node.data])
		case NumberLit:
			v = a.numbers[node.data]
		default:
			return false
		}
		if !checkNumberBounds(v, n.numbers.flags, n.numbers.min, n.numbers.max) {
			return false
		}
	}
	if n.length.flags != 0 {
		if node.kind != StringLit {
			return false
		}
		l := int64(utf8.RuneCountInString(a.StringValue(StringID(node.data))))
		if !checkLenBounds(l, n.length.flags, n.length.min, n.length.max) {
			return false
		}
	}
	return true
}

func checkIntBounds(v int64, flags boundFlags, minv, maxv int64) bool {
	if flags&hasMin != 0 {
		if v < minv || v == minv && flags&minInclusive == 0 {
			return false
		}
	}
	if flags&hasMax != 0 {
		if v > maxv || v == maxv && flags&maxInclusive == 0 {
			return false
		}
	}
	return true
}

func checkNumberBounds(v float64, flags boundFlags, minv, maxv float64) bool {
	if math.IsNaN(v) {
		return false
	}
	if flags&hasMin != 0 {
		if v < minv || v == minv && flags&minInclusive == 0 {
			return false
		}
	}
	if flags&hasMax != 0 {
		if v > maxv || v == maxv && flags&maxInclusive == 0 {
			return false
		}
	}
	return true
}

func checkLenBounds(v int64, flags boundFlags, minv, maxv int64) bool {
	if flags&hasMin != 0 {
		if v < minv || v == minv && flags&minInclusive == 0 {
			return false
		}
	}
	if flags&hasMax != 0 {
		if v > maxv || v == maxv && flags&maxInclusive == 0 {
			return false
		}
	}
	return true
}
