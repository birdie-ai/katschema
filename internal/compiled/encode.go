package compiled

import (
	"encoding/binary"
	"math"

	"github.com/cespare/xxhash"
)

const encodingVersion byte = 1

func sum64(p []byte) uint64 { return xxhash.Sum64(p) }

func put32(p []byte, v int32) []byte {
	return binary.LittleEndian.AppendUint32(p, uint32(v))
}

func put64(p []byte, v int64) []byte {
	return binary.LittleEndian.AppendUint64(p, uint64(v))
}

func putu64(p []byte, v uint64) []byte {
	return binary.LittleEndian.AppendUint64(p, v)
}

func putf64(p []byte, v float64) []byte {
	return putu64(p, math.Float64bits(canonicalFloat(v)))
}

func putstr(p []byte, s string) []byte {
	p = put32(p, int32(len(s)))
	return append(p, s...)
}

func putBigInt(p []byte, negative bool, mag []byte) []byte {
	p = append(p, 0xff)
	if negative {
		p = append(p, 1)
	} else {
		p = append(p, 0)
	}
	p = put32(p, int32(len(mag)))
	return append(p, mag...)
}

func (a *Arena) putIntLiteral(p []byte, id TypeID) []byte {
	if id == 0 {
		return put64(p, 0)
	}
	n := a.Node(id)
	if n.kind != IntLit {
		panic(n.kind)
	}
	ikind := integerKind(n.data)
	// NOTE(i4k): we don't need to encode ikind because AFAICS there's no ambiguity
	// as we only store the big int if v > math.MaxInt64 and then their representation
	// is always a mismatch.
	if ikind == sint {
		return put64(p, a.int64(n.data))
	}
	v, mag := a.bigIntData(n.data)
	return putBigInt(p, v.negative(), mag)
}

// this function is weird but it canonicalize as +0 in the case of -0.
// TODO(i4k): should we normalize NaNs?
func canonicalFloat(v float64) float64 {
	if v == 0 {
		return 0
	}
	return v
}

func floatEqual(a, b float64) bool {
	return math.Float64bits(canonicalFloat(a)) == math.Float64bits(canonicalFloat(b))
}
