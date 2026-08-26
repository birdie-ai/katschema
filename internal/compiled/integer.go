package compiled

import (
	"bytes"
	"math"
	"math/big"
)

type intKind bool

const (
	sint intKind = false
	bint intKind = true
)

// bigInt stores the magnitude of an integer that does not fit into int64.
// The bigInt bytes are big-endian encoded. Check [math/big.Int.Bytes](https://cs.opensource.google/go/go/+/refs/tags/go1.27.0:src/math/big/int.go;l=607-616).
// It holds the offset and size in the arena largeIntBytes.
// The size is negative for negative values.
// NOTE(i4k): this design intention is to handle large integers using a compact 64 bits
// data structure.
type bigInt struct {
	off  int32
	size int32
}

func integerKind(data int32) intKind {
	switch {
	case data > 0:
		return sint
	case data < 0:
		return bint
	default:
		panic("unreachable")
	}
}

func makeBigInt(off int32, mag []byte, negative bool) bigInt {
	size := int32(len(mag))
	if negative {
		size = -size
	}
	return bigInt{off: off, size: size}
}

func (v bigInt) negative() bool { return v.size < 0 }

func (v bigInt) magnitudeLen() int32 {
	if v.size < 0 {
		return -v.size
	}
	return v.size
}

func (a *Arena) isIntSmall(data int32) bool {
	return data > 0
}

func (a *Arena) int64(data int32) int64 {
	if data <= 0 {
		panic("programming error")
	}
	return a.ints[data]
}

func (a *Arena) bigInt(data int32) *big.Int {
	if data >= 0 {
		panic("programming error")
	}
	v, mag := a.bigIntData(data)
	var vbig big.Int
	vbig.SetBytes(mag)
	if v.negative() {
		vbig.Neg(&vbig)
	}
	return &vbig
}

func (a *Arena) bigIntData(data int32) (bigInt, []byte) {
	if data >= 0 {
		panic("unreachable")
	}
	i := -data
	v := a.bigInts[i]
	n := v.magnitudeLen()
	return v, a.bigIntBytes[v.off : v.off+n]
}

func (a *Arena) equalBigInts(data int32, negative bool, mag []byte) bool {
	v, vmag := a.bigIntData(data)
	return v.negative() == negative && bytes.Equal(vmag, mag)
}

func (a *Arena) compareInts(x, y int32) int {
	xkind := integerKind(x)
	ykind := integerKind(y)
	switch {
	case xkind == sint && ykind == sint:
		xs := a.int64(x)
		ys := a.int64(y)
		if xs < ys {
			return -1
		}
		if xs > ys {
			return 1
		}
		return 0
	case xkind == sint:
		yv, _ := a.bigIntData(y)
		if yv.negative() {
			return 1
		}
		return -1
	case ykind == sint:
		xv, _ := a.bigIntData(x)
		if xv.negative() {
			return -1
		}
		return 1
	}
	// both are big
	xv, xmag := a.bigIntData(x)
	yv, ymag := a.bigIntData(y)
	if xv.negative() != yv.negative() {
		if xv.negative() {
			return -1
		}
		return 1
	}

	cmp := compareMagnitude(xmag, ymag)
	if xv.negative() {
		return -cmp
	}
	return cmp
}

// NOTE(i4k): this only works out because we are using the big.Int internal big-endian representation.
func compareMagnitude(a, b []byte) int {
	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}
	return bytes.Compare(a, b)
}

func (a *Arena) nextIntLiteral(id TypeID) TypeID {
	return a.intLiteralAdd64(id, 1)
}

func (a *Arena) prevIntLiteral(id TypeID) TypeID {
	return a.intLiteralAdd64(id, -1)
}

func (a *Arena) intLiteralAdd64(id TypeID, v int64) TypeID {
	if v == 0 {
		return id
	}
	n := a.Node(id)
	if n.kind != IntLit {
		panic(n.kind.String())
	}

	var x *big.Int
	if integerKind(n.data) == sint {
		val := a.int64(n.data)

		if v > 0 {
			if val <= math.MaxInt64-v {
				return a.internInt(val + v)
			}
		} else {
			if val >= math.MinInt64-v {
				return a.internInt(val + v)
			}
		}

		x = big.NewInt(val)
	} else {
		x = a.bigInt(n.data)
	}
	return a.internBigInt(x.Add(x, big.NewInt(v)))
}
