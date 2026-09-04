package compiled

import (
	"bytes"
	"math/big"
	"strconv"
)

// realValue stores the coefficient of an exact decimal and its base-10
// exponent. The value is coefficient * 10^exp. The coefficient is kept as
// canonical decimal digits because real values only need exact comparison and
// conversion at this stage; expanding a large exponent into a big.Int would
// be both wasteful and potentially unbounded.
type realValue struct {
	off  int32
	size int32 // negative for negative values; zero is always stored positive
	exp  int64
}

func makeRealValue(off int32, digits []byte, negative bool, exp int64) realValue {
	size := int32(len(digits))
	if negative {
		size = -size
	}
	return realValue{off: off, size: size, exp: exp}
}

func (v realValue) negative() bool { return v.size < 0 }

func (v realValue) digitsLen() int32 {
	if v.size < 0 {
		return -v.size
	}
	return v.size
}

func (a *Arena) realData(data int32) (realValue, []byte) {
	if data <= 0 || int(data) >= len(a.reals) {
		panic("invalid real atom")
	}
	v := a.reals[data]
	n := v.digitsLen()
	return v, a.realBytes[v.off : v.off+n]
}

func (a *Arena) internRawReal(negative bool, digits []byte, exp int64) TypeID {
	if len(digits) == 0 {
		panic("empty real coefficient")
	}

	if len(digits) == 1 && digits[0] == '0' {
		negative = false
		exp = 0
	} else {
		trailing := 0
		for len(digits) > 1 && digits[len(digits)-1] == '0' {
			digits = digits[:len(digits)-1]
			trailing++
		}
		// The parser bounds the canonical exponent, so this addition cannot
		// overflow for parser-produced values.
		exp += int64(trailing)
	}

	a.scratch = a.scratch[:0]
	a.scratch = append(a.scratch, encodingVersion, byte(RealAtom))
	if negative {
		a.scratch = append(a.scratch, 1)
	} else {
		a.scratch = append(a.scratch, 0)
	}
	a.scratch = put64(a.scratch, exp)
	a.scratch = put32(a.scratch, int32(len(digits)))
	a.scratch = append(a.scratch, digits...)
	fp := a.hash(a.scratch)
	if id := a.find(fp, func(id TypeID) bool {
		n := a.nodes[id]
		if n.kind != RealAtom {
			return false
		}
		v, oldDigits := a.realData(n.data)
		return v.negative() == negative && v.exp == exp && bytes.Equal(oldDigits, digits)
	}); id != 0 {
		return id
	}

	off := int32(len(a.realBytes))
	a.realBytes = append(a.realBytes, digits...)
	i := int32(len(a.reals))
	a.reals = append(a.reals, makeRealValue(off, digits, negative, exp))
	return a.appendNode(Node{kind: RealAtom, data: i}, fp, a.hashHead[fp])
}

func (a *Arena) realNumber(id TypeID) (decimalNumber, bool) {
	n := a.Node(id)
	switch n.kind {
	case RealAtom:
		v, digits := a.realData(n.data)
		return decimalNumber{negative: v.negative(), digits: digits, exp: v.exp}, true
	case IntAtom:
		var raw string
		if n.data > 0 {
			raw = strconv.FormatInt(a.int64(n.data), 10)
		} else {
			raw = a.bigInt(n.data).String()
		}
		negative := len(raw) > 0 && raw[0] == '-'
		if negative {
			raw = raw[1:]
		}
		return decimalNumber{negative: negative, digits: []byte(raw)}, true
	default:
		return decimalNumber{}, false
	}
}

type decimalNumber struct {
	negative bool
	digits   []byte
	exp      int64
}

func (v decimalNumber) zero() bool {
	return len(v.digits) == 1 && v.digits[0] == '0'
}

func compareDecimalNumbers(x, y decimalNumber) int {
	xzero, yzero := x.zero(), y.zero()
	if xzero || yzero {
		switch {
		case xzero && yzero:
			return 0
		case xzero:
			if y.negative {
				return 1
			}
			return -1
		default:
			if x.negative {
				return -1
			}
			return 1
		}
	}
	if x.negative != y.negative {
		if x.negative {
			return -1
		}
		return 1
	}

	cmp := compareDecimalMagnitude(x, y)
	if x.negative {
		return -cmp
	}
	return cmp
}

func compareDecimalMagnitude(x, y decimalNumber) int {
	// Compare the position of the most significant digit first. big.Int is
	// used only for this exponent arithmetic, avoiding overflow at the edge
	// of int64 while keeping coefficient storage compact.
	var xe, ye big.Int
	xe.SetInt64(x.exp)
	xe.Add(&xe, big.NewInt(int64(len(x.digits))))
	ye.SetInt64(y.exp)
	ye.Add(&ye, big.NewInt(int64(len(y.digits))))
	if cmp := xe.Cmp(&ye); cmp != 0 {
		return cmp
	}

	n := max(len(x.digits), len(y.digits))
	for i := 0; i < n; i++ {
		xd, yd := byte('0'), byte('0')
		if i < len(x.digits) {
			xd = x.digits[i]
		}
		if i < len(y.digits) {
			yd = y.digits[i]
		}
		if xd < yd {
			return -1
		}
		if xd > yd {
			return 1
		}
	}
	return 0
}

func (a *Arena) compareRealNumbers(x, y TypeID) int {
	xv, xok := a.realNumber(x)
	yv, yok := a.realNumber(y)
	if !xok || !yok {
		panic("non-real atom")
	}
	return compareDecimalNumbers(xv, yv)
}

func (a *Arena) realFromIntAtom(id TypeID) TypeID {
	v, ok := a.realNumber(id)
	if !ok || a.Node(id).kind != IntAtom {
		panic("unexpected")
	}
	return a.internRawReal(v.negative, v.digits, v.exp)
}

func (a *Arena) realString(id TypeID) string {
	v, ok := a.realNumber(id)
	if !ok {
		panic("not a real atom")
	}
	return decimalNumberString(v)
}

func decimalNumberString(v decimalNumber) string {
	if v.zero() {
		return "0"
	}
	var out []byte
	if v.negative {
		out = append(out, '-')
	}
	out = append(out, v.digits...)
	if v.exp != 0 {
		out = append(out, 'e')
		out = strconv.AppendInt(out, v.exp, 10)
	}
	return string(out)
}

func (a *Arena) realToFloat64(id TypeID) (float64, bool) {
	v, err := strconv.ParseFloat(a.realString(id), 64)
	return canonicalFloat(v), err == nil
}

func (a *Arena) floatAtom(id TypeID) (TypeID, bool) {
	switch a.Node(id).kind {
	case FloatAtom:
		return id, true
	case IntAtom:
		if !a.isIntSmall(a.Node(id).data) {
			return 0, false
		}
		return a.internFloat(float64(a.int64(a.Node(id).data))), true
	case RealAtom:
		v, ok := a.realToFloat64(id)
		if !ok {
			return 0, false
		}
		return a.internFloat(v), true
	default:
		return 0, false
	}
}
