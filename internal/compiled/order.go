package compiled

import (
	"cmp"
	"unicode/utf8"
)

// Subtype is the semantic lattice order a <= b within this arena.
// In other words it reports whether every value accepted by a is also accepted by b.
//
// This method strictly tell if the property described above holds and
// under any circunstance it should have special handling for types using any
// other characteristics of them.
func (a *Arena) Subtype(left, right TypeID) bool {
	return a.Type(left).SubtypeOf(a.Type(right))
}

// SubtypeOf compares references in their original arenas. Identity is a fast
// path only when both the owning arena and the node/view are identical.
func (a Type) SubtypeOf(b Type) bool {
	// NOTE(i4k): basic lattice ordering fundamentals:
	//   (never) <= literals... <= types... <= ... <= (any)
	//
	// Subtype(a, a)         == true because a accepts all a values.
	// Subtype((never), ...) == true because (never) accepts no value which b also accepts.
	// Subtype(..., (any))   == true because b accepts everything.
	// Subtype((any), ...)   == false because otherwise the other type is (any) itself.
	// Subtype(..., (never)) == false because never accepts no value, so accepts none of a values.

	ak, bk := a.Kind(), b.Kind()
	if ak == Invalid || bk == Invalid {
		return false
	}
	if a == b || ak == Never || bk == Any {
		return true
	}
	if ak == Any || bk == Never {
		return false
	}
	if ak == Sum {
		for _, id := range a.a.sum(a.id) {
			if !a.a.Type(id).SubtypeOf(b) {
				return false
			}
		}
		return true
	}
	if ak == Refined && a.constraint().id == 0 {
		return a.Base().SubtypeOf(b)
	}
	if bk == Refined && b.constraint().id == 0 {
		return a.SubtypeOf(b.Base())
	}
	if bk == Sum {
		// T <= (A | B) == T <= A || T <= B
		for _, id := range b.a.sum(b.id) {
			if a.SubtypeOf(b.a.Type(id)) {
				return true
			}
		}
		return false
	}
	if bk == Refined && b.constraint().data().flags == constraintLen {
		if bounds, ok := a.containerLength(); ok {
			d := b.constraint().data()
			return a.SubtypeOf(b.Base()) &&
				lowerIntStronger(bounds.flags, bounds.min, d.lenFlags, d.lenMin) &&
				upperIntStronger(bounds.flags, bounds.max, d.lenFlags, d.lenMax)
		}
	}
	if ak == Refined {
		ab := a.Base()
		if bk == Refined && ab.Equivalent(b.Base()) {
			return a.constraint().subset(b.constraint())
		}
		if atom, ok := a.literal(); ok {
			return atom.SubtypeOf(b)
		}
		return ab.SubtypeOf(b)
	}
	if bk == Refined {
		atom, ok := a.literal()
		return ok && atom.SubtypeOf(b.Base()) && b.constraint().accepts(atom)
	}
	switch ak {
	case Null, Bool, Int, Real, String:
		return ak == bk || ak == Int && bk == Real
	case BoolAtom:
		return bk == Bool || bk == BoolAtom && compareAtoms(a, b) == 0
	case IntAtom:
		return bk == Int || bk == Real || (bk == IntAtom || bk == RealAtom) && compareAtoms(a, b) == 0
	case RealAtom:
		return bk == Real || bk == RealAtom && compareAtoms(a, b) == 0
	case StringAtom:
		return bk == String || bk == StringAtom && compareAtoms(a, b) == 0
	case List:
		return bk == List && a.Element().SubtypeOf(b.Element())
	case Tuple:
		if bk == List {
			for i := 0; i < a.Len(); i++ {
				if !a.At(i).SubtypeOf(b.Element()) {
					return false
				}
			}
			return true
		}
		if bk != Tuple || a.Len() != b.Len() {
			return false
		}
		for i := 0; i < a.Len(); i++ {
			if !a.At(i).SubtypeOf(b.At(i)) {
				return false
			}
		}
		return true
	case Object:
		return bk == Object && a.objectSubtype(b)
	default:
		panic(ak)
	}
}

func (t Type) containerLength() (lenBounds, bool) {
	switch t.Kind() {
	case List:
		return lenBounds{flags: hasMin}, true
	case Tuple:
		n := int64(t.Len())
		return lenBounds{flags: hasMin | hasMax, min: n, max: n}, true
	case Object:
		fields := t.Fields()
		bounds := lenBounds{flags: hasMin | hasMax, max: int64(fields.Len())}
		for i := 0; i < fields.Len(); i++ {
			if !fields.At(i).Optional() {
				bounds.min++
			}
		}
		return bounds, true
	case Refined:
		bounds, ok := t.Base().containerLength()
		if !ok {
			return lenBounds{}, false
		}
		d := t.constraint().data()
		if d.flags&constraintLen != 0 {
			if d.lenFlags&hasMin != 0 && d.lenMin > bounds.min {
				bounds.min = d.lenMin
			}
			if d.lenFlags&hasMax != 0 && (bounds.flags&hasMax == 0 || d.lenMax < bounds.max) {
				bounds.flags |= hasMax
				bounds.max = d.lenMax
			}
		}
		return bounds, true
	default:
		return lenBounds{}, false
	}
}

func (a Type) Equivalent(b Type) bool {
	return a.SubtypeOf(b) && b.SubtypeOf(a)
}

func (t Type) literal() (Type, bool) {
	if t.view != nil {
		return t.a.Type(0), false
	}
	id, ok := t.a.Literal(t.id)
	return t.a.Type(id), ok
}

func (a Type) objectSubtype(b Type) bool {
	af, bf := a.Fields(), b.Fields()
	for i := 0; i < af.Len(); i++ {
		f := af.At(i)
		g, ok := bf.Get(f.Name())
		if !ok || f.Optional() && !g.Optional() || !f.Type().SubtypeOf(g.Type()) {
			return false
		}
	}
	for i := 0; i < bf.Len(); i++ {
		g := bf.At(i)
		if g.Optional() {
			continue
		}
		f, ok := af.Get(g.Name())
		if !ok || f.Optional() {
			return false
		}
	}
	return true
}

func (a *Arena) findField(fields []Field, name string) int {
	lo, hi := 0, len(fields)
	for lo < hi {
		m := int(uint(lo+hi) >> 1)
		if a.StringValue(fields[m].Name) < name {
			lo = m + 1
		} else {
			hi = m
		}
	}
	if lo < len(fields) && a.StringValue(fields[lo].Name) == name {
		return lo
	}
	return -1
}

type constraintRef struct {
	a  *Arena
	id ConstraintID
}

func (c constraintRef) data() constraintData {
	if c.id == 0 {
		return constraintData{}
	}
	return c.a.constraints[c.id]
}

func (a *Arena) atomSatisfiesConstraint(atom TypeID, id ConstraintID) bool {
	return (constraintRef{a: a, id: id}).accepts(a.Type(atom))
}

func (c constraintRef) accepts(atom Type) bool {
	if c.id == 0 {
		return true
	}
	d := c.data()
	kind := atom.Kind()
	if d.flags&constraintInt != 0 {
		if kind != IntAtom || !c.contains(atom, d.intFlags, d.intMin, d.intMax, false) {
			return false
		}
	}
	if d.flags&constraintReal != 0 {
		if kind != IntAtom && kind != RealAtom || !c.contains(atom, d.realFlags, d.realMin, d.realMax, true) {
			return false
		}
	}
	if d.flags&constraintFloatConv != 0 {
		if kind != IntAtom && kind != RealAtom || !atom.a.realCanBeFloat(atom.id, d.format) {
			return false
		}
	}
	if d.flags&constraintFloat != 0 {
		if kind != IntAtom && kind != RealAtom {
			return false
		}
		v, ok := atom.a.realToFloat64(atom.id)
		if !ok || !checkFloatBounds(v, d.floatFlags, d.numMin, d.numMax) {
			return false
		}
	}
	if d.flags&constraintLen != 0 {
		var length int64
		switch kind {
		case StringAtom:
			length = int64(utf8.RuneCountInString(atom.a.StringValue(StringID(atom.a.Node(atom.id).data))))
		case Tuple, Object:
			length = int64(atom.Len())
		default:
			return false
		}
		if !checkIntBounds(length, d.lenFlags, d.lenMin, d.lenMax) {
			return false
		}
	}
	if d.flags&constraintEnum != 0 {
		switch kind {
		case Null, BoolAtom, IntAtom, RealAtom, StringAtom:
		default:
			return false
		}
		for _, id := range c.a.constraintEnums[d.enum.off : d.enum.off+d.enum.len] {
			if compareAtoms(atom, c.a.Type(id)) == 0 {
				return true
			}
		}
		return false
	}
	return true
}

func (c constraintRef) contains(atom Type, flags boundFlags, min, max TypeID, continuous bool) bool {
	if flags&hasMin != 0 {
		v := compareAtoms(atom, c.a.Type(min))
		if v < 0 || v == 0 && continuous && flags&minInclusive == 0 {
			return false
		}
	}
	if flags&hasMax != 0 {
		v := compareAtoms(atom, c.a.Type(max))
		if v > 0 || v == 0 && continuous && flags&maxInclusive == 0 {
			return false
		}
	}
	return true
}

func (a constraintRef) subset(b constraintRef) bool {
	if b.id == 0 || a == b {
		return true
	}
	if a.id == 0 {
		return false
	}
	ad, bd := a.data(), b.data()
	if ad.flags&constraintEnum != 0 {
		for _, id := range a.a.constraintEnums[ad.enum.off : ad.enum.off+ad.enum.len] {
			if !b.accepts(a.a.Type(id)) {
				return false
			}
		}
		return true
	}
	if bd.flags&constraintEnum != 0 {
		return false
	}
	if bd.flags&constraintInt != 0 &&
		(ad.flags&constraintInt == 0 || !atomBoundsSubset(a, ad.intFlags, ad.intMin, ad.intMax, b, bd.intFlags, bd.intMin, bd.intMax, false)) {
		return false
	}
	if bd.flags&constraintReal != 0 &&
		(ad.flags&constraintReal == 0 || !atomBoundsSubset(a, ad.realFlags, ad.realMin, ad.realMax, b, bd.realFlags, bd.realMin, bd.realMax, true)) {
		return false
	}
	if bd.flags&constraintFloatConv != 0 && (ad.flags&constraintFloatConv == 0 || !floatFmtSubset(ad.format, bd.format)) {
		return false
	}
	if bd.flags&constraintFloat != 0 && (ad.flags&constraintFloat == 0 || !numberBoundsSubset(ad, bd)) {
		return false
	}
	if bd.flags&constraintLen != 0 && (ad.flags&constraintLen == 0 || !lenBoundsSubset(ad, bd)) {
		return false
	}
	return true
}

func atomBoundsSubset(a constraintRef, af boundFlags, amin, amax TypeID, b constraintRef, bf boundFlags, bmin, bmax TypeID, continuous bool) bool {
	if bf&hasMin != 0 {
		if af&hasMin == 0 {
			return false
		}
		c := compareAtoms(a.a.Type(amin), b.a.Type(bmin))
		if c < 0 || c == 0 && continuous && bf&minInclusive == 0 && af&minInclusive != 0 {
			return false
		}
	}
	if bf&hasMax != 0 {
		if af&hasMax == 0 {
			return false
		}
		c := compareAtoms(a.a.Type(amax), b.a.Type(bmax))
		if c > 0 || c == 0 && continuous && bf&maxInclusive == 0 && af&maxInclusive != 0 {
			return false
		}
	}
	return true
}

func compareAtoms(a, b Type) int {
	if a == b {
		return 0
	}
	if a.a == b.a {
		return a.a.compareAtom(a.id, b.id)
	}
	an, bn := a.a.Node(a.id), b.a.Node(b.id)
	if an.kind == IntAtom && bn.kind == IntAtom {
		if an.data > 0 && bn.data > 0 {
			return cmp.Compare(a.a.int64(an.data), b.a.int64(bn.data))
		}
		if an.data > 0 {
			v, _ := b.a.bigIntData(bn.data)
			if v.negative() {
				return 1
			}
			return -1
		}
		if bn.data > 0 {
			v, _ := a.a.bigIntData(an.data)
			if v.negative() {
				return -1
			}
			return 1
		}
		av, am := a.a.bigIntData(an.data)
		bv, bm := b.a.bigIntData(bn.data)
		if av.negative() != bv.negative() {
			if av.negative() {
				return -1
			}
			return 1
		}
		c := compareMagnitude(am, bm)
		if av.negative() {
			return -c
		}
		return c
	}
	if (an.kind == IntAtom || an.kind == RealAtom) && (bn.kind == IntAtom || bn.kind == RealAtom) {
		av, _ := a.a.realNumber(a.id)
		bv, _ := b.a.realNumber(b.id)
		return compareDecimalNumbers(av, bv)
	}
	if an.kind != bn.kind {
		return cmp.Compare(an.kind, bn.kind)
	}
	switch an.kind {
	case Null:
		return 0
	case BoolAtom:
		return cmp.Compare(an.data, bn.data)
	case StringAtom:
		return cmp.Compare(a.a.StringValue(StringID(an.data)), b.a.StringValue(StringID(bn.data)))
	default:
		panic("not a scalar atom")
	}
}

func floatFmtSubset(a, b floatFmt) bool {
	return b == noFmt || a != noFmt && (a == b || a == f32Fmt && b == f64Fmt)
}

func numberBoundsSubset(a, b constraintData) bool {
	return lowerFloatStronger(a.floatFlags, a.numMin, b.floatFlags, b.numMin) &&
		upperFloatStronger(a.floatFlags, a.numMax, b.floatFlags, b.numMax)
}

func lenBoundsSubset(a, b constraintData) bool {
	return lowerIntStronger(a.lenFlags, a.lenMin, b.lenFlags, b.lenMin) &&
		upperIntStronger(a.lenFlags, a.lenMax, b.lenFlags, b.lenMax)
}

func lowerIntStronger(af boundFlags, av int64, bf boundFlags, bv int64) bool {
	return bf&hasMin == 0 || af&hasMin != 0 && av >= bv
}

func upperIntStronger(af boundFlags, av int64, bf boundFlags, bv int64) bool {
	return bf&hasMax == 0 || af&hasMax != 0 && av <= bv
}

func lowerFloatStronger(af boundFlags, av float64, bf boundFlags, bv float64) bool {
	if bf&hasMin == 0 {
		return true
	}
	if af&hasMin == 0 || av < bv {
		return false
	}
	return av > bv || bf&minInclusive != 0 || af&minInclusive == 0
}

func upperFloatStronger(af boundFlags, av float64, bf boundFlags, bv float64) bool {
	if bf&hasMax == 0 {
		return true
	}
	if af&hasMax == 0 || av > bv {
		return false
	}
	return av < bv || bf&maxInclusive != 0 || af&maxInclusive == 0
}
