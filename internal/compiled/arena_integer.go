package compiled

import (
	"math"
	"math/big"
	"strconv"
)

func (a *Arena) intBuiltinType(name string) (TypeID, bool) {
	if len(name) < 4 {
		return 0, false
	}
	var min, max TypeID
	switch name {
	default:
		return 0, false
	case "int8":
		min, max = a.internInt(math.MinInt8), a.internInt(math.MaxInt8)
	case "int16":
		min, max = a.internInt(math.MinInt16), a.internInt(math.MaxInt16)
	case "int32":
		min, max = a.internInt(math.MinInt32), a.internInt(math.MaxInt32)
	case "int64":
		min, max = a.internInt(math.MinInt64), a.internInt(math.MaxInt64)
	case "uint8":
		min, max = a.internInt(0), a.internInt(math.MaxUint8)
	case "uint16":
		min, max = a.internInt(0), a.internInt(math.MaxUint16)
	case "uint32":
		min, max = a.internInt(0), a.internInt(math.MaxUint32)
	case "uint", "uint64":
		var v big.Int
		v.SetUint64(math.MaxUint64)
		min, max = a.internInt(0), a.internBigInt(&v)
	}
	n := normConstraint{ints: intBounds{flags: hasMin | hasMax, min: min, max: max}}
	return a.internRefined(a.Int(), a.internConstraint(n)), true
}

func (a *Arena) rawIntWithinBounds(raw string, r intBounds) bool {
	if v, err := strconv.ParseInt(raw, 10, 64); err == nil {
		if r.flags&hasMin != 0 {
			bn := a.Node(r.min)
			switch integerKind(bn.data) {
			case sint:
				if v < a.int64(bn.data) {
					return false
				}
			case bint:
				bv, _ := a.bigIntData(bn.data)
				if !bv.negative() {
					// NOTE(i4k): you know why, right?
					return false
				}
			}
		}
		if r.flags&hasMax != 0 {
			bn := a.Node(r.max)
			switch integerKind(bn.data) {
			case sint:
				if v > a.int64(bn.data) {
					return false
				}
			case bint:
				bv, _ := a.bigIntData(bn.data)
				if bv.negative() {
					return false
				}
			}
		}
		return true
	}

	var v big.Int
	_, ok := v.SetString(raw, 10)
	if !ok {
		panic("unreachable")
	}
	if r.flags&hasMin != 0 {
		if cmp := a.compareBigIntData(&v, r.min); cmp < 0 {
			return false
		}
	}
	if r.flags&hasMax != 0 {
		if cmp := a.compareBigIntData(&v, r.max); cmp > 0 {
			return false
		}
	}
	return true
}

func (a *Arena) compareBigIntData(x *big.Int, yID TypeID) int {
	yn := a.Node(yID)
	if integerKind(yn.data) == sint {
		var yb big.Int
		yb.SetInt64(a.int64(yn.data))
		return x.Cmp(&yb)
	}
	yb, mag := a.bigIntData(yn.data)
	xneg := x.Sign() < 0
	if xneg != yb.negative() {
		if xneg {
			return -1
		}
		return 1
	}
	cmp := compareMagnitude(x.Bytes(), mag)
	if xneg {
		cmp = -cmp
	}
	return cmp
}
