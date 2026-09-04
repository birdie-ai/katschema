package compiled

import (
	"math"
	"strconv"
)

// floatFmt is an intrinsic conversion constraint for a mathematical real.
// The source value remains exact; the format determines which finite values
// can be converted to the target IEEE-754 width.
type floatFmt uint8

const (
	noFmt floatFmt = iota
	f32Fmt
	f64Fmt
)

func (a *Arena) internFloatFormat(format floatFmt) TypeID {
	if format != f32Fmt && format != f64Fmt {
		panic("invalid float format")
	}
	return a.internRefined(a.real, a.internConstraint(normConstraint{format: format}))
}

func (a *Arena) realCanBeFloat(id TypeID, format floatFmt) bool {
	v, ok := a.realNumber(id)
	return ok && decimalCanBeFloat(v, format)
}

func decimalCanBeFloat(v decimalNumber, format floatFmt) bool {
	var bits int
	switch format {
	case f32Fmt:
		bits = 32
	case f64Fmt:
		bits = 64
	default:
		return false
	}

	// NOTE(i4k): The conversion is intentionally allowed to be inexact. Comparing the
	// decimal with the exact binary value would incorrectly reject values such
	// as 0.1, which are valid inputs for the target IEEE-754 format.
	parsed, err := strconv.ParseFloat(decimalNumberString(v), bits)
	return err == nil && !math.IsNaN(parsed) && !math.IsInf(parsed, 0)
}
