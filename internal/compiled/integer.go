package compiled

import "bytes"

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

func (a *Arena) int64(data int32) (int64, bool) {
	if data <= 0 {
		return 0, false
	}
	return a.ints[data], true
}

func (a *Arena) bigInt(data int32) (bigInt, []byte, bool) {
	if data >= 0 {
		return bigInt{}, nil, false
	}
	i := -data
	v := a.bigInts[i]
	n := v.magnitudeLen()
	return v, a.bigIntBytes[v.off : v.off+n], true
}

func (a *Arena) equalBigInts(data int32, negative bool, mag []byte) bool {
	v, old, ok := a.bigInt(data)
	return ok && v.negative() == negative && bytes.Equal(old, mag)
}

func (a *Arena) compareInts(x, y int32) int {
	xs, xSmall := a.int64(x)
	ys, ySmall := a.int64(y)
	switch {
	case xSmall && ySmall:
		if xs < ys {
			return -1
		}
		if xs > ys {
			return 1
		}
		return 0
	case xSmall:
		yv, _, _ := a.bigInt(y)
		if yv.negative() {
			return 1
		}
		return -1
	case ySmall:
		xv, _, _ := a.bigInt(y)
		if xv.negative() {
			return -1
		}
		return 1
	}
	// both are big
	xv, xb, _ := a.bigInt(x)
	yv, yb, _ := a.bigInt(y)
	if xv.negative() != yv.negative() {
		if xv.negative() {
			return -1
		}
		return 1
	}

	cmp := compareMagnitude(xb, yb)
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
