package compiled

import (
	"encoding/binary"
	"math"

	"github.com/cespare/xxhash"
)

const encodingVersion byte = 1

func sum64(p []byte) uint64 { return xxhash.Sum64(p) }

func appendInt32(p []byte, v int32) []byte {
	return binary.LittleEndian.AppendUint32(p, uint32(v))
}

func appendInt64(p []byte, v int64) []byte {
	return binary.LittleEndian.AppendUint64(p, uint64(v))
}

func appendUint64(p []byte, v uint64) []byte {
	return binary.LittleEndian.AppendUint64(p, v)
}

func appendFloat64(p []byte, v float64) []byte {
	return appendUint64(p, math.Float64bits(canonicalFloat(v)))
}

func appendString(p []byte, s string) []byte {
	p = appendInt32(p, int32(len(s)))
	return append(p, s...)
}

func canonicalFloat(v float64) float64 {
	if v == 0 {
		return 0
	}
	return v
}

func floatEqual(a, b float64) bool {
	return math.Float64bits(canonicalFloat(a)) == math.Float64bits(canonicalFloat(b))
}
