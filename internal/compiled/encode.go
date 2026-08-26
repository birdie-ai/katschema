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
