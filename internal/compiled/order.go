package compiled

// Subtype reports whether every value accepted by a is also accepted by b.
//
// This method strictly tell if the property described above holds and
// under any circunstance it should have special handling for types using any
// other characteristics of them.
//
// It is the semantic lattice order a <= b.
func (x *Arena) Subtype(a, b TypeID) bool {
	// NOTE(i4k): basic lattice ordering fundamentals:
	//   (never) <= literals... <= types... <= ... <= (any)

	// Subtype(a, a) 		 == true because a accepts all a values.
	// Subtype((never), ...) == true because (never) accepts no value which b also accepts.
	// Subtype(..., (any))   == true because b accepts everything.
	if a == b || a == x.neverID || b == x.anyID {
		return true
	}

	// Subtype((any), ...)   == false because otherwise the other type is (any) itself.
	// Subtype(..., (never)) == false because never accepts no value, so accepts none of a values.
	if a == 0 || b == 0 || a == x.anyID || b == x.neverID {
		return false
	}

	an, bn := x.Node(a), x.Node(b)
	if an.kind == Refined {
		ar := x.refinements[an.data]
		if bn.kind == Refined {
			br := x.refinements[bn.data]
			if ar.base == br.base {
				return x.constraintSubset(ar.constraint, br.constraint)
			}
		}
		return x.Subtype(ar.base, b)
	}
	if bn.kind == Refined {
		br := x.refinements[bn.data]
		return x.isLiteral(a) && x.Subtype(a, br.base) && x.literalSatisfiesConstraint(a, br.constraint)
	}

	switch an.kind {
	case Null:
		return bn.kind == Null
	case BoolLit:
		return bn.kind == Bool
	case IntLit:
		return bn.kind == Int || bn.kind == Float
	case FloatLit:
		return bn.kind == Float
	case StringLit:
		return bn.kind == String
	case Int:
		return bn.kind == Float
	case List:
		return bn.kind == List && x.Subtype(TypeID(an.data), TypeID(bn.data))
	case Tuple:
		if bn.kind == List {
			for _, elem := range x.tuple(a) {
				if !x.Subtype(elem, TypeID(bn.data)) {
					return false
				}
			}
			return true
		}
		if bn.kind != Tuple {
			return false
		}
		av, bv := x.tuple(a), x.tuple(b)
		if len(av) != len(bv) {
			return false
		}
		for i := range av {
			if !x.Subtype(av[i], bv[i]) {
				return false
			}
		}
		return true
	case Object:
		return bn.kind == Object && x.objectSubtype(a, b)
	}
	return false
}

func (x *Arena) objectSubtype(a, b TypeID) bool {
	af, bf := x.objectFields(a), x.objectFields(b)

	// Every field A may emit must be accepted by B.
	for _, f := range af {
		j := x.findField(bf, x.StringValue(f.Name))
		if j < 0 {
			return false
		}
		g := bf[j]
		if f.Optional() && !g.Optional() {
			return false
		}
		if !x.Subtype(f.Type, g.Type) {
			return false
		}
	}

	// Every field required by B must be required by A.
	for _, f := range bf {
		if f.Optional() {
			continue
		}
		j := x.findField(af, x.StringValue(f.Name))
		if j < 0 || af[j].Optional() {
			return false
		}
	}
	return true
}

func (x *Arena) findField(fields []Field, name string) int {
	lo, hi := 0, len(fields)
	for lo < hi {
		m := int(uint(lo+hi) >> 1)
		if x.StringValue(fields[m].Name) < name {
			lo = m + 1
		} else {
			hi = m
		}
	}
	if lo < len(fields) && x.StringValue(fields[lo].Name) == name {
		return lo
	}
	return -1
}

func (x *Arena) isLiteral(id TypeID) bool {
	switch x.Node(id).kind {
	case Null, BoolLit, IntLit, FloatLit, StringLit:
		return true
	}
	return false
}

func (x *Arena) literalSatisfiesConstraint(lit TypeID, id ConstraintID) bool {
	if id == 0 {
		return true
	}
	d := x.constraints[id]
	n := normConstraint{}
	if d.flags&constraintInt != 0 {
		n.ints = intBounds{flags: d.intFlags, min: d.intMin, max: d.intMax}
	}
	if d.flags&constraintFloat != 0 {
		n.floats = floatBounds{flags: d.floatFlags, min: d.numMin, max: d.numMax}
	}
	if d.flags&constraintLen != 0 {
		n.length = intBounds{flags: d.lenFlags, min: d.lenMin, max: d.lenMax}
	}
	if !x.literalSatisfiesNormConstraint(lit, n) {
		return false
	}
	if d.flags&constraintEnum != 0 {
		for _, v := range x.constraintEnums[d.enum.off : d.enum.off+d.enum.len] {
			if v == lit {
				return true
			}
		}
		return false
	}
	return true
}

func (x *Arena) constraintSubset(a, b ConstraintID) bool {
	if a == b || b == 0 {
		return true
	}
	if a == 0 {
		return false
	}
	ad, bd := x.constraints[a], x.constraints[b]

	if ad.flags&constraintEnum != 0 {
		for _, lit := range x.constraintEnums[ad.enum.off : ad.enum.off+ad.enum.len] {
			if !x.literalSatisfiesConstraint(lit, b) {
				return false
			}
		}
		return true
	}
	if bd.flags&constraintEnum != 0 {
		return false
	}

	if bd.flags&constraintInt != 0 {
		if ad.flags&constraintInt == 0 || !intBoundsSubset(ad, bd) {
			return false
		}
	}
	if bd.flags&constraintFloat != 0 {
		if ad.flags&constraintFloat == 0 || !numberBoundsSubset(ad, bd) {
			return false
		}
	}
	if bd.flags&constraintLen != 0 {
		if ad.flags&constraintLen == 0 || !lenBoundsSubset(ad, bd) {
			return false
		}
	}
	return true
}

func intBoundsSubset(a, b constraintData) bool {
	return lowerIntStronger(a.intFlags, a.intMin, b.intFlags, b.intMin) &&
		upperIntStronger(a.intFlags, a.intMax, b.intFlags, b.intMax)
}

func numberBoundsSubset(a, b constraintData) bool {
	return lowerNumberStronger(a.floatFlags, a.numMin, b.floatFlags, b.numMin) &&
		upperNumberStronger(a.floatFlags, a.numMax, b.floatFlags, b.numMax)
}

func lenBoundsSubset(a, b constraintData) bool {
	return lowerIntStronger(a.lenFlags, a.lenMin, b.lenFlags, b.lenMin) &&
		upperIntStronger(a.lenFlags, a.lenMax, b.lenFlags, b.lenMax)
}

func lowerIntStronger(af boundFlags, av int64, bf boundFlags, bv int64) bool {
	if bf&hasMin == 0 {
		return true
	}
	if af&hasMin == 0 || av < bv {
		return false
	}
	if av > bv {
		return true
	}
	return bf&minInclusive != 0 || af&minInclusive == 0
}

func upperIntStronger(af boundFlags, av int64, bf boundFlags, bv int64) bool {
	if bf&hasMax == 0 {
		return true
	}
	if af&hasMax == 0 || av > bv {
		return false
	}
	if av < bv {
		return true
	}
	return bf&maxInclusive != 0 || af&maxInclusive == 0
}

func lowerNumberStronger(af boundFlags, av float64, bf boundFlags, bv float64) bool {
	if bf&hasMin == 0 {
		return true
	}
	if af&hasMin == 0 || av < bv {
		return false
	}
	if av > bv {
		return true
	}
	return bf&minInclusive != 0 || af&minInclusive == 0
}

func upperNumberStronger(af boundFlags, av float64, bf boundFlags, bv float64) bool {
	if bf&hasMax == 0 {
		return true
	}
	if af&hasMax == 0 || av > bv {
		return false
	}
	if av < bv {
		return true
	}
	return bf&maxInclusive != 0 || af&maxInclusive == 0
}
