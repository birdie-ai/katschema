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
	min   TypeID
	max   TypeID
}

type realBounds struct {
	flags boundFlags
	min   TypeID
	max   TypeID
}

type lenBounds struct {
	flags boundFlags
	min   int64
	max   int64
}

type floatBounds struct {
	flags boundFlags
	min   float64
	max   float64
}

type normConstraint struct {
	ints   intBounds
	reals  realBounds
	format floatFmt
	floats floatBounds
	length lenBounds
	enum   []TypeID // nil means no enum restriction; empty non-nil means impossible.
}

// NOTE(i4k): using bound flags to avoid dealing with ambiguities when using the same type
// data constraint type.

// NOTE(i4k): minInclusive and maxInclusive was designed to be used by all numeric types but
// design was broken because for discrete types like integers we can normalize all bounds to
// inclusive with huge benefits. A Len constraint is also discrete, so in the end inclusivity
// flags only matter to real numbers.

type boundFlags uint8

const (
	hasMin boundFlags = 1 << iota
	minInclusive
	hasMax
	maxInclusive
)

// NOTE(i4k): DO NOT ADD POINTERS HERE.
// We use Go struct equality for comparing interned constraints and `enum` field is already
// problematic because enums are not interned, so two different range32 offsets can
// actually point to semantically same enum values, breaking the interning, so we
// empty the enum range before comparing the struct. Check the constraintEqual() function.
type constraintData struct {
	flags uint8

	intFlags boundFlags
	intMin   TypeID
	intMax   TypeID

	realFlags boundFlags
	realMin   TypeID
	realMax   TypeID
	format    floatFmt

	floatFlags boundFlags
	numMin     float64
	numMax     float64

	lenFlags boundFlags
	lenMin   int64
	lenMax   int64

	enum range32
}

const (
	constraintInt uint8 = 1 << iota
	constraintFloat
	constraintLen
	constraintEnum
	constraintReal
	constraintFloatConv
)

func (c *compiler) extendNormalizeConstraints(base TypeID, initial normConstraint, clauses []ast.NodeID) (ct normConstraint, impossible bool, err error) {
	n := normalizer{c: c, base: base, n: initial}
	for _, id := range clauses {
		if c.t.Node(id).Kind() != ast.Constraint {
			continue
		}
		err := n.consume(c.t.Constraint(id))
		if err != nil {
			return normConstraint{}, false, err
		}
	}
	return n.finish()
}

type normalizer struct {
	c          *compiler
	base       TypeID
	n          normConstraint
	impossible bool
}

func (n *normalizer) consume(id ast.NodeID) error {
	id = n.ungroup(id)
	node := n.c.t.Node(id)
	if node.Kind() != ast.Binary {
		return n.c.errorf(id, "unsupported constraint expression %s", node.Kind())
	}

	b := n.c.t.Binary(id)
	if b.Op == token.And {
		// C1 && C2
		if err := n.consume(b.Left); err != nil {
			return err
		}
		return n.consume(b.Right)
	}

	// TODO(i4k): I know this is not complete but enough for now.
	// Here we normalize binary OR branches only into ENUMs but this is incomplete as we can
	// have OR of arbitrary predicates.
	if b.Op == token.Or {
		enum, err := n.consumeEnumOr(id)
		if err != nil {
			return err
		}
		n.addEnum(enum)
		return nil
	}

	if isRangeOp(b.Op) {
		values, ops := n.flattenOrder(b)
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
	return n.c.errorf(id, "unsupported constraint operator %s", b.Op)
}

func (n *normalizer) consumeEnumOr(id ast.NodeID) ([]TypeID, error) {
	id = n.ungroup(id)
	if n.c.t.Node(id).Kind() != ast.Binary {
		return nil, n.c.errorf(id, "unsupported expression in OR constraint")
	}

	b := n.c.t.Binary(id)
	if b.Op == token.Or {
		left, err := n.consumeEnumOr(b.Left)
		if err != nil {
			return nil, err
		}

		right, err := n.consumeEnumOr(b.Right)
		if err != nil {
			return nil, err
		}

		left = append(left, right...)
		return n.c.a.sortUniqueTypes(left), nil
	}

	switch b.Op {
	case token.Eq:
		return n.equalityValues(b.Left, b.Right)

	case token.In:
		return n.inValues(b.Left, b.Right)
	}

	return nil, n.c.errorf(
		id,
		"OR currently requires equality or in constraints",
	)
}

// TODO(i4k): handle nested grouping or reject AST with reduntant groups.
func (n *normalizer) ungroup(id ast.NodeID) ast.NodeID {
	for n.c.t.Node(id).Kind() == ast.Group {
		id = n.c.t.Group(id)
	}
	return id
}

func isRangeOp(op token.Kind) bool {
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

// NOTE(i4k): this could seem complex but it's actually quite simple.
// it transforms: (int, ((0 <= x) <= 100)) ==> (int, 0 <= x <= 100)
// for the case above it returns: values=[0, x, 100], ops=[<=, <=]
// then later (check the compare() function) we check:
//
//	values[i] ops[i] values[i+1]
//
// materializing the checks:
//
//	0 <= x
//	x <= 100
func (n *normalizer) flattenOrder(b ast.BinaryData) ([]ast.NodeID, []token.Kind) {
	left := n.ungroup(b.Left)
	if n.c.t.Node(left).Kind() == ast.Binary {
		lb := n.c.t.Binary(left)
		if isRangeOp(lb.Op) {
			values, ops := n.flattenOrder(lb)
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
	return n.c.errorf(left, "constraint comparison must reference x or len(x)")
}

// equal canonicalize equality checks as enums. This is great because it composes
// nicely: (int, x == 1, x == 2) and (int, x IN [1, 2]) are semantically the same.
func (n *normalizer) equal(left, right ast.NodeID) error {
	values, err := n.equalityValues(left, right)
	if err != nil {
		return err
	}

	n.addEnum(values)
	return nil
}

func (n *normalizer) equalityValues(left, right ast.NodeID) ([]TypeID, error) {
	left = n.ungroup(left)
	right = n.ungroup(right)

	var id ast.NodeID

	switch {
	case n.isX(left):
		id = right
	case n.isX(right):
		id = left
	default:
		return nil, n.c.errorf(
			left,
			"equality constraint must compare x with a literal",
		)
	}

	lit, err := n.literal(id)
	if err != nil {
		return nil, err
	}

	if !n.c.a.atomCompat(n.base, lit) {
		return nil, n.c.errorf(
			id,
			"literal %s is not compatible with %s",
			n.c.a.Node(lit).Kind(),
			n.c.a.Node(n.base).Kind(),
		)
	}

	return []TypeID{lit}, nil
}

func (n *normalizer) inValues(left, right ast.NodeID) ([]TypeID, error) {
	left = n.ungroup(left)
	right = n.ungroup(right)

	if !n.isX(left) {
		return nil, n.c.errorf(left, "left side of in must be x")
	}

	if n.c.t.Node(right).Kind() != ast.List {
		return nil, n.c.errorf(
			right,
			"right side of in must be a literal array",
		)
	}

	values := n.c.t.List(right)
	enum := make([]TypeID, 0, len(values))

	for _, v := range values {
		id, err := n.literal(v)
		if err != nil {
			return nil, err
		}

		if !n.c.a.atomCompat(n.base, id) {
			return nil, n.c.errorf(
				v,
				"enum literal is not compatible with %s",
				n.c.a.Node(n.base).Kind(),
			)
		}

		enum = append(enum, id)
	}

	return n.c.a.sortUniqueTypes(enum), nil
}

func (n *normalizer) inSet(left, right ast.NodeID) error {
	values, err := n.inValues(left, right)
	if err != nil {
		return err
	}

	n.addEnum(values)
	return nil
}

func (n *normalizer) literal(id ast.NodeID) (TypeID, error) {
	id = n.ungroup(id)
	switch n.c.t.Node(id).Kind() {
	case ast.Null, ast.Bool, ast.Int, ast.String:
		t, err := n.c.scalarLiteralAtom(id)
		if err != nil {
			return 0, err
		}
		return t, nil
	case ast.Decimal:
		t, err := n.c.scalarLiteralAtom(id)
		return t, err
	default:
		return 0, n.c.errorf(id, "constraint requires a scalar literal")
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

	litKind := n.c.t.Node(literal).Kind()
	switch kind {
	case Int:
		if litKind != ast.Int {
			return n.c.errorf(literal, "int constraint requires an integer bound")
		}
		v, err := n.c.intAtom(literal, n.c.t.Int(literal))
		if err != nil {
			return err
		}
		n.applyInt(op, v)
		return nil
	case Real:
		var v TypeID
		switch litKind {
		case ast.Int:
			intAtom, err := n.c.intAtom(literal, n.c.t.Int(literal))
			if err != nil {
				return err
			}
			v = n.c.a.realFromIntAtom(intAtom)
		case ast.Decimal:
			var err error
			v, err = n.c.realAtom(literal, n.c.t.Decimal(literal))
			if err != nil {
				return err
			}
		default:
			return n.c.errorf(literal, "real constraint requires a numeric bound")
		}
		n.applyReal(op, v)
		return nil
	default:
		return n.c.errorf(literal, "numeric comparison is invalid for %s", kind)
	}
}

func (n *normalizer) lengthBound(op token.Kind, literal ast.NodeID) error {
	kind := n.c.a.baseKind(n.base)
	switch kind {
	case String, List, Tuple, Object:
	default:
		return n.c.errorf(literal, "len(x) is invalid for %s", kind)
	}
	litKind := n.c.t.Node(literal).Kind()
	if litKind != ast.Int {
		return n.c.errorf(literal, "length comparison requires integer right-hand-side but got %s", litKind)
	}
	raw := n.c.t.Int(literal)
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || v < 0 {
		return n.c.errorf(literal, "length bound must be a non-negative int64")
	}
	n.applyLen(op, v)
	return nil
}

func (n *normalizer) applyInt(op token.Kind, v TypeID) {
	// NOTE(i4k): integers are discrete so we can normalize:
	//   (int, x > 0)   -> (int, x >= 1)
	//   (int, x < 100) -> (int, x <= 99)
	// and then internally they are all inclusive bounds.
	switch op {
	case token.Gt:
		v = n.c.a.nextIntAtom(v)
		op = token.Ge
	case token.Lt:
		v = n.c.a.prevIntAtom(v)
		op = token.Le
	}

	r := &n.n.ints
	switch op {
	case token.Ge:
		if r.flags&hasMin == 0 || n.c.a.compareAtom(v, r.min) > 0 {
			r.min = v
			r.flags |= hasMin
		}
	case token.Le:
		if r.flags&hasMax == 0 || n.c.a.compareAtom(v, r.max) < 0 {
			r.max = v
			r.flags |= hasMax
		}
	}
}

func (n *normalizer) applyReal(op token.Kind, v TypeID) {
	r := &n.n.reals
	switch op {
	default:
		panic(op)
	case token.Gt, token.Ge:
		inclusive := op == token.Ge
		cmp := 1
		if r.flags&hasMin != 0 {
			cmp = n.c.a.compareAtom(v, r.min)
		}
		if r.flags&hasMin == 0 || cmp > 0 || cmp == 0 && !inclusive && r.flags&minInclusive != 0 {
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
		cmp := -1
		if r.flags&hasMax != 0 {
			cmp = n.c.a.compareAtom(v, r.max)
		}
		if r.flags&hasMax == 0 || cmp < 0 || cmp == 0 && !inclusive && r.flags&maxInclusive != 0 {
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
	switch op {
	case token.Gt:
		if v == math.MaxInt64 {
			n.impossible = true
			return
		}
		v++
		op = token.Ge
	case token.Lt:
		if v == math.MinInt64 {
			n.impossible = true
			return
		}
		v--
		op = token.Le
	}
	r := &n.n.length
	switch op {
	case token.Ge:
		if r.flags&hasMin == 0 || v > r.min {
			r.min = v
			r.flags |= hasMin
		}
	case token.Le:
		if r.flags&hasMax == 0 || v < r.max {
			r.max = v
			r.flags |= hasMax
		}
	}
}

func (n *normalizer) addEnum(v []TypeID) {
	if n.c.a.baseKind(n.base) == Real {
		for i, id := range v {
			if n.c.a.Node(id).kind == IntAtom {
				v[i] = n.c.a.realFromIntAtom(id)
			}
		}
	}
	v = n.c.a.sortUniqueTypes(v)
	if n.n.enum == nil {
		n.n.enum = v
		return
	}
	out := make([]TypeID, 0, min(len(n.n.enum), len(v)))
	i, j := 0, 0
	for i < len(n.n.enum) && j < len(v) {
		a, b := n.n.enum[i], v[j]
		cmp := n.c.a.compareAtom(a, b)
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

func (n *normalizer) finish() (ct normConstraint, impossible bool, err error) {
	if n.impossible {
		return normConstraint{}, true, nil
	}
	if n.c.a.impossibleIntBounds(n.n.ints) || n.c.a.impossibleRealBounds(n.n.reals) || impossibleFloatBounds(n.n.floats) || impossibleLenBounds(n.n.length) {
		return normConstraint{}, true, nil
	}

	kind := n.c.a.baseKind(n.base)
	if n.n.enum == nil {
		switch kind {
		case Int:
			if n.c.a.singleInt(n.n.ints) {
				n.n.enum = []TypeID{n.n.ints.min}
				n.n.ints = intBounds{}
			}
		case Real:
			if n.c.a.singleReal(n.n.reals) {
				n.n.enum = []TypeID{n.n.reals.min}
				n.n.reals = realBounds{}
			}
		}
	}

	if n.n.enum != nil {
		out := n.n.enum[:0]
		for _, id := range n.n.enum {
			if n.c.a.atomSatisfiesNormConstraint(id, n.n) {
				out = append(out, id)
			}
		}
		n.n.enum = out
		if len(out) == 0 {
			return normConstraint{}, true, nil
		}
		// NOTE(i4k): Once schema has enums the other restrictions are redundant.
		// We can check for clauses conflicting with the enums later and fail in such cases.
		n.n.ints = intBounds{}
		n.n.reals = realBounds{}
		n.n.format = noFmt
		n.n.floats = floatBounds{}
		n.n.length = lenBounds{}
	}
	return n.n, false, nil
}

func (a *Arena) impossibleIntBounds(r intBounds) bool {
	if r.flags&(hasMin|hasMax) != hasMin|hasMax {
		return false
	}
	return a.compareAtom(r.min, r.max) > 0
}

func (a *Arena) impossibleRealBounds(r realBounds) bool {
	if r.flags&(hasMin|hasMax) != hasMin|hasMax {
		return false
	}
	cmp := a.compareAtom(r.min, r.max)
	if cmp > 0 {
		return true
	}
	return cmp == 0 && (r.flags&minInclusive == 0 || r.flags&maxInclusive == 0)
}

func impossibleLenBounds(r lenBounds) bool {
	if r.flags&(hasMin|hasMax) != hasMin|hasMax {
		return false
	}
	return r.min > r.max
}

func impossibleFloatBounds(r floatBounds) bool {
	if r.flags&(hasMin|hasMax) != hasMin|hasMax {
		return false
	}
	if r.min > r.max {
		return true
	}
	return r.min == r.max && (r.flags&minInclusive == 0 || r.flags&maxInclusive == 0)
}

func (a *Arena) singleInt(r intBounds) bool {
	return r.flags&(hasMin|hasMax) == hasMin|hasMax && r.min == r.max
}

func (a *Arena) singleReal(r realBounds) bool {
	return r.flags&(hasMin|minInclusive|hasMax|maxInclusive) == hasMin|minInclusive|hasMax|maxInclusive && a.compareAtom(r.min, r.max) == 0
}

func (a *Arena) internConstraint(n normConstraint) ConstraintID {
	a.scratch = a.scratch[:0]
	a.scratch = append(a.scratch, encodingVersion, 0xc1)
	if n.ints.flags != 0 {
		a.scratch = append(a.scratch, constraintInt, byte(n.ints.flags))
		a.scratch = a.putIntAtom(a.scratch, n.ints.min)
		a.scratch = a.putIntAtom(a.scratch, n.ints.max)
	}
	if n.reals.flags != 0 {
		a.scratch = append(a.scratch, constraintReal, byte(n.reals.flags))
		a.scratch = a.putRealAtom(a.scratch, n.reals.min)
		a.scratch = a.putRealAtom(a.scratch, n.reals.max)
	}
	if n.format != noFmt {
		a.scratch = append(a.scratch, constraintFloatConv, byte(n.format))
	}
	if n.floats.flags != 0 {
		a.scratch = append(a.scratch, constraintFloat, byte(n.floats.flags))
		a.scratch = putf64(a.scratch, n.floats.min)
		a.scratch = putf64(a.scratch, n.floats.max)
	}
	if n.length.flags != 0 {
		a.scratch = append(a.scratch, constraintLen, byte(n.length.flags))
		a.scratch = put64(a.scratch, n.length.min)
		a.scratch = put64(a.scratch, n.length.max)
	}
	if n.enum != nil {
		a.scratch = append(a.scratch, constraintEnum)
		a.scratch = put32(a.scratch, int32(len(n.enum)))
		for _, id := range n.enum {
			a.scratch = putu64(a.scratch, a.Fingerprint(id))
		}
	}
	fp := a.hash(a.scratch)

	for id := a.constraintHead[fp]; id != 0; id = a.constraintNext[id] {
		if a.constraintEqual(id, n) {
			return id
		}
	}

	d := initConstraintDataFromNorm(n)
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

// TODO(i4k): this function sucks and it should be as simple as `return a.constraints[i] == n`
// but that's not possible right now because `d.enum` is a range32 offsets. Maybe we should
// also intern enums separately and then store an EnumID in the constraintData, then we
// solve this with Go struct equality.
func (a *Arena) constraintEqual(id ConstraintID, n normConstraint) bool {
	got := a.constraints[id]
	want := initConstraintDataFromNorm(n)

	enum := got.enum
	got.enum = range32{}
	want.enum = range32{}

	// NOTE(i4k): HERE BE DRAGONS!
	// I don't like this but the alternative is also not great. I think ideally we should
	// intern enums but not a priority now.
	if got != want {
		return false
	}
	if n.enum == nil {
		return true
	}

	old := a.constraintEnums[enum.off : enum.off+enum.len]
	return equalTypeIDs(old, n.enum)
}

func emptyConstraint(n normConstraint) bool {
	return n.ints.flags == 0 && n.reals.flags == 0 && n.format == noFmt && n.floats.flags == 0 && n.length.flags == 0 && n.enum == nil
}

func initConstraintDataFromNorm(n normConstraint) constraintData {
	var d constraintData

	if n.ints.flags != 0 {
		d.flags |= constraintInt
		d.intFlags = n.ints.flags
		d.intMin = n.ints.min
		d.intMax = n.ints.max
	}

	if n.reals.flags != 0 {
		d.flags |= constraintReal
		d.realFlags = n.reals.flags
		d.realMin = n.reals.min
		d.realMax = n.reals.max
	}

	if n.format != noFmt {
		d.flags |= constraintFloatConv
		d.format = n.format
	}

	if n.floats.flags != 0 {
		d.flags |= constraintFloat
		d.floatFlags = n.floats.flags
		d.numMin = n.floats.min
		d.numMax = n.floats.max
	}

	if n.length.flags != 0 {
		d.flags |= constraintLen
		d.lenFlags = n.length.flags
		d.lenMin = n.length.min
		d.lenMax = n.length.max
	}

	if n.enum != nil {
		d.flags |= constraintEnum
		// NOTE(i4k): enum intentionally left zero here
		// this is a hack to avoid a complex equality check!
		// This only works while the constraintData struct is (mostly pointer free).
		// Check the constraintEqual() method.
	}

	return d
}

func (a *Arena) constraintFingerprint(id ConstraintID) uint64 {
	if id <= 0 || int(id) >= len(a.constraintHash) {
		return 0
	}
	return a.constraintHash[id]
}

func (a *Arena) sortUniqueTypes(v []TypeID) []TypeID {
	sort.Slice(v, func(i, j int) bool { return a.compareAtom(v[i], v[j]) < 0 })
	out := v[:0]
	for _, id := range v {
		if len(out) == 0 || a.compareAtom(out[len(out)-1], id) != 0 {
			out = append(out, id)
		}
	}
	return out
}

// compareAtom gives finite atom sets an order independent of arena IDs and fingerprints.
// This is used as sorting Less function (see [sort.Interface]) but also when shifting sorted
// values.
func (a *Arena) compareAtom(x, y TypeID) int {
	xn, yn := a.Node(x), a.Node(y)
	if (xn.kind == IntAtom || xn.kind == RealAtom) && (yn.kind == IntAtom || yn.kind == RealAtom) {
		return a.compareRealNumbers(x, y)
	}
	if xn.kind < yn.kind {
		return -1
	}
	if xn.kind > yn.kind {
		return 1
	}
	switch xn.kind {
	case Null:
		return 0
	case BoolAtom:
		if xn.data < yn.data {
			return -1
		}
		if xn.data > yn.data {
			return 1
		}
	case IntAtom:
		return a.compareInts(xn.data, yn.data)
	case StringAtom:
		xv, yv := a.StringValue(StringID(xn.data)), a.StringValue(StringID(yn.data))
		if xv < yv {
			return -1
		}
		if xv > yv {
			return 1
		}
	default:
		panic(xn.kind)
	}
	return 0
}

func (a *Arena) baseKind(id TypeID) Kind {
	for a.Node(id).kind == Refined {
		id = a.refinements[a.Node(id).data].base
	}
	return a.Node(id).kind
}

func (a *Arena) atomCompat(base, atom TypeID) bool {
	bk := a.baseKind(base)
	ln := a.Node(atom)
	lk := ln.kind
	switch bk {
	case Any:
		return true
	case Null:
		return lk == Null
	case Bool:
		return lk == BoolAtom
	case Int:
		return lk == IntAtom
	case Real:
		return lk == IntAtom || lk == RealAtom
	case String:
		return lk == StringAtom
	}
	return false
}

func (a *Arena) atomSatisfiesNormConstraint(id TypeID, n normConstraint) bool {
	node := a.Node(id)
	if n.ints.flags != 0 {
		if node.kind != IntAtom || !a.checkIntAtomBounds(id, n.ints) {
			return false
		}
	}
	if n.reals.flags != 0 {
		if (node.kind != IntAtom && node.kind != RealAtom) || !a.checkRealAtomBounds(id, n.reals) {
			return false
		}
	}
	if n.format != noFmt {
		if (node.kind != IntAtom && node.kind != RealAtom) || !a.realCanBeFloat(id, n.format) {
			return false
		}
	}
	if n.floats.flags != 0 {
		var v float64
		switch node.kind {
		case IntAtom:
			var ok bool
			v, ok = a.realToFloat64(id)
			if !ok {
				return false
			}
		case RealAtom:
			var ok bool
			v, ok = a.realToFloat64(id)
			if !ok {
				return false
			}
		default:
			return false
		}
		if !checkFloatBounds(v, n.floats.flags, n.floats.min, n.floats.max) {
			return false
		}
	}
	if n.length.flags != 0 {
		if node.kind != StringAtom {
			return false
		}
		l := int64(utf8.RuneCountInString(a.StringValue(StringID(node.data))))
		if !checkIntBounds(l, n.length.flags, n.length.min, n.length.max) {
			return false
		}
	}
	return true
}

func (a *Arena) intConstraintNorm(id ConstraintID) normConstraint {
	d := a.constraints[id]
	n := normConstraint{}
	if d.flags&constraintInt != 0 {
		n.ints = intBounds{flags: d.intFlags, min: d.intMin, max: d.intMax}
	}
	return n
}

func (a *Arena) realConstraintNorm(id ConstraintID) normConstraint {
	d := a.constraints[id]
	n := normConstraint{}
	if d.flags&constraintReal != 0 {
		n.reals = realBounds{flags: d.realFlags, min: d.realMin, max: d.realMax}
	}
	if d.flags&constraintFloatConv != 0 {
		n.format = d.format
	}
	if d.enum.len > 0 {
		n.enum = a.constraintEnums[d.enum.off : d.enum.off+d.enum.len]
	}
	return n
}

func (a *Arena) checkIntAtomBounds(id TypeID, r intBounds) bool {
	n := a.Node(id)
	if n.kind != IntAtom {
		return false
	}
	if r.flags&hasMin != 0 && a.compareInts(n.data, a.Node(r.min).data) < 0 {
		return false
	}
	if r.flags&hasMax != 0 && a.compareInts(n.data, a.Node(r.max).data) > 0 {
		return false
	}
	return true
}

func (a *Arena) checkRealAtomBounds(id TypeID, r realBounds) bool {
	if a.Node(id).kind != IntAtom && a.Node(id).kind != RealAtom {
		return false
	}
	if r.flags&hasMin != 0 {
		cmp := a.compareAtom(id, r.min)
		if cmp < 0 || cmp == 0 && r.flags&minInclusive == 0 {
			return false
		}
	}
	if r.flags&hasMax != 0 {
		cmp := a.compareAtom(id, r.max)
		if cmp > 0 || cmp == 0 && r.flags&maxInclusive == 0 {
			return false
		}
	}
	return true
}

func checkIntBounds(v int64, flags boundFlags, minv, maxv int64) bool {
	if flags&hasMin != 0 {
		if v < minv {
			return false
		}
	}
	if flags&hasMax != 0 {
		if v > maxv {
			return false
		}
	}
	return true
}

func checkFloatBounds(v float64, flags boundFlags, minv, maxv float64) bool {
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
