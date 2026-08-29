package parser

import (
	"errors"
	"fmt"
	"math"
)

// DecimalParts is a scaled exact decimal representation.
// It uses the scaled decimal representation below:
//
//	Digits * 10^Exp
//
// The Digits will never have leading or trailing zeroes, except zero itself.
type DecimalParts struct {
	Exp    int64
	Digits []byte
	Neg    bool
}

var (
	ErrParseDecimal            = errors.New("parsing decimal number")
	ErrParseDecimalExpMissing  = fmt.Errorf("%w: missing exponent digits", ErrParseDecimal)
	ErrParseDecimalExpOverflow = fmt.Errorf("%w: exponent overflow", ErrParseDecimal)
)

// Decimal parses raw into a canonical exact decimal.
// The `buf` is a scratch storage for the digits coefficients.
// This is function is designed to be allocation free given that cap(buf) == len(raw).
// Note that buf can grow and in such case the new pointer is returned in the
// [DecimalParts.Digits] slice.
//
// Advanced usage:
// The raw and buf may share the same backing array but only when buf starts at raw
// is supported, and if used in this way, it's an allocation-free destructive parser.
//
//	dec, err := parse.Decimal(raw, raw[:0])
//
// after this call, raw is mutated and dec.Digits owns the array.
func Decimal(raw []byte, buf []byte) (DecimalParts, error) {
	if len(raw) == 0 {
		return DecimalParts{}, fmt.Errorf("%w: missing digits", ErrParseDecimal)
	}

	buf = buf[:0]
	pos := 0
	neg := false
	if raw[0] == '-' {
		neg = true
		pos++

		if pos == len(raw) {
			return DecimalParts{}, fmt.Errorf("%w: missing digits", ErrParseDecimal)
		}
	}

	if !isDigit(raw[pos]) {
		return DecimalParts{}, fmt.Errorf("%w: missing integer digits", ErrParseDecimal)
	}

	// NOTE(i4k): this handles JSON number special case of forbidding numbers with a
	// leading zero, eg.: 01
	if raw[pos] == '0' {
		buf = append(buf, '0')
		pos++
		if pos < len(raw) && isDigit(raw[pos]) {
			return DecimalParts{}, fmt.Errorf("%w: malformed number with leading zero", ErrParseDecimal)
		}
	} else {
		for pos < len(raw) && isDigit(raw[pos]) {
			buf = append(buf, raw[pos])
			pos++
		}
	}

	// NOTE(i4k): we accumulate both integer and fractional into a single coefficient
	// digits slice.

	fracLen := 0
	if pos < len(raw) && raw[pos] == '.' {
		pos++
		fracStart := pos
		for pos < len(raw) && isDigit(raw[pos]) {
			buf = append(buf, raw[pos])
			pos++
		}
		fracLen = pos - fracStart
		if fracLen == 0 {
			return DecimalParts{}, fmt.Errorf("%w: missing fracional part", ErrParseDecimal)
		}
	}

	// NOTE(i4k): here we canonicalize the digits before parsing the exponent.
	// eg.:
	//   1.0000e-1 == 1e-1
	//   1.00001   == 100001e-5
	// This is important because we want to check for exp overflow in a single place.

	start := 0
	for start < len(buf) && buf[start] == '0' {
		start++
	}

	allZeroes := start == len(buf)
	if allZeroes {
		buf = buf[:1]
		buf[0] = '0'
	} else if start != 0 {
		copy(buf, buf[start:])
		buf = buf[:len(buf)-start]
	}

	adjust := int64(0)
	if !allZeroes {
		trailing := 0
		for len(buf) > 1 && buf[len(buf)-1] == '0' {
			buf = buf[:len(buf)-1]
			trailing++
		}
		adjust = int64(trailing) - int64(fracLen)
	}

	exp := adjust // exp inferred from digits
	if pos < len(raw) {
		// NOTE(i4k): at this point only 'e' and 'E' are expected.

		if c := raw[pos]; c != 'e' && c != 'E' {
			return DecimalParts{}, fmt.Errorf("%w: unexpected %c", ErrParseDecimal, c)
		}
		pos++
		if allZeroes {
			n, err := scanExp(raw[pos:])
			if err != nil {
				return DecimalParts{}, err
			}
			pos += n
		} else {
			var n int
			var err error
			exp, n, err = parseExp(raw[pos:], adjust)
			if err != nil {
				return DecimalParts{}, err
			}
			pos += n
		}
	}
	if pos != len(raw) {
		return DecimalParts{}, fmt.Errorf("%w: unexpected %c", ErrParseDecimal, raw[pos])
	}
	if allZeroes {
		return DecimalParts{Digits: buf}, nil
	}
	return DecimalParts{Neg: neg, Digits: buf, Exp: exp}, nil
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func parseExp(raw []byte, adjust int64) (int64, int, error) {
	if len(raw) == 0 {
		return 0, 0, ErrParseDecimalExpMissing
	}
	pos := 0
	neg := false
	switch raw[pos] {
	case '-':
		neg = true
		fallthrough
	case '+':
		pos++
	}
	if pos == len(raw) || !isDigit(raw[pos]) {
		return 0, 0, ErrParseDecimalExpMissing
	}

	limit := expLimit(neg, adjust)
	var magnitude uint64
	for pos < len(raw) && isDigit(raw[pos]) {
		digit := uint64(raw[pos] - '0')
		if magnitude > limit/10 || (magnitude == limit/10 && digit > limit%10) {
			return 0, 0, ErrParseDecimalExpOverflow
		}
		magnitude = magnitude*10 + digit
		pos++
	}

	return applyExp(adjust, neg, magnitude), pos, nil
}

func scanExp(raw []byte) (int, error) {
	if len(raw) == 0 {
		return 0, ErrParseDecimalExpMissing
	}
	pos := 0
	if c := raw[pos]; c == '+' || c == '-' {
		pos++
	}
	if pos == len(raw) || !isDigit(raw[pos]) {
		return 0, ErrParseDecimalExpMissing
	}
	for pos < len(raw) && isDigit(raw[pos]) {
		pos++
	}
	return pos, nil
}

// expLimit computes the maximum uint64 that an explicit exponent can have.
// by explicit exponent I mean what user places after '[eE]' in the source.
// This is a variable number because we are canonicalizing the digits. Eg.:
//
//	1.0001 == 10001e-4 (adjust == -4)
//
// so when user add explicit exponent, we need to take that into consideration
// when checking for exponent overflow.
func expLimit(neg bool, adjust int64) uint64 {
	const minMagnitude = uint64(1) << 63
	if neg {
		if adjust >= 0 {
			return minMagnitude + uint64(adjust)
		}
		return minMagnitude - absExp(adjust)
	}
	if adjust >= 0 {
		return uint64(math.MaxInt64 - adjust)
	}
	return uint64(math.MaxInt64) + absExp(adjust)
}

// applyExp computes the final exponent based on adjust and the explicit user exponent.
// Note that at this point we know adjust+mag does not overflow.
func applyExp(adjust int64, neg bool, magnitude uint64) int64 {
	const minMagnitude = uint64(1) << 63
	if !neg {
		if adjust >= 0 {
			return adjust + int64(magnitude)
		}
		absVal := absExp(adjust)
		if magnitude >= absVal {
			return int64(magnitude - absVal)
		}
		d := absVal - magnitude
		if d == minMagnitude {
			return math.MinInt64
		}
		return -int64(d)
	}
	if adjust <= 0 {
		d := absExp(adjust) + magnitude
		if d == minMagnitude {
			return math.MinInt64
		}
		return -int64(d)
	}
	absVal := uint64(adjust)
	if magnitude <= absVal {
		return int64(absVal - magnitude)
	}
	d := magnitude - absVal
	if d == minMagnitude {
		return math.MinInt64
	}
	return -int64(d)
}

func absExp(v int64) uint64 {
	if v >= 0 {
		return uint64(v)
	}
	return uint64(-(v + 1)) + 1
}
