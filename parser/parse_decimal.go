package parser

import (
	"errors"
	"fmt"
	"math"

	"github.com/birdie-ai/katschema/parser/ast"
)

var ErrParseDecimal = errors.New("parsing decimal number")

func Decimal(raw []byte) (ast.Decimal, error) {
	if len(raw) == 0 {
		return ast.Decimal{}, fmt.Errorf("%w: missing digits", ErrParseDecimal)
	}

	neg := false
	switch raw[0] {
	case '-':
		neg = true
		raw = raw[1:]
	case '+':
		raw = raw[1:]
	}

	if len(raw) == 0 {
		return ast.Decimal{}, fmt.Errorf("%w: missing digits", ErrParseDecimal)
	}

	pos := 0
	for pos < len(raw) && isDigit(raw[pos]) {
		pos++
	}
	if pos == 0 {
		return ast.Decimal{}, fmt.Errorf("%w: missing integer digits", ErrParseDecimal)
	}

	// NOTE(i4k): Reject 00, 01, etc., but allow 0, 0.1, 0e1.
	if pos > 1 && raw[0] == '0' {
		return ast.Decimal{}, fmt.Errorf("%w: malformed number with leading zero", ErrParseDecimal)
	}

	intEnd := pos
	fracStart := 0
	fracEnd := 0

	if pos < len(raw) && raw[pos] == '.' {
		fracStart = pos + 1
		pos = fracStart

		for pos < len(raw) && isDigit(raw[pos]) {
			pos++
		}

		if pos == fracStart {
			return ast.Decimal{}, fmt.Errorf("%w: missing fractional digits", ErrParseDecimal)
		}

		fracEnd = pos
	}

	fracLen := fracEnd - fracStart

	// Parse the explicit exponent before mutating raw.
	var expPart int64
	if pos < len(raw) {
		if raw[pos] != 'e' && raw[pos] != 'E' {
			return ast.Decimal{}, fmt.Errorf("%w: unexpected %c", ErrParseDecimal, raw[pos])
		}

		pos++
		if pos == len(raw) {
			return ast.Decimal{}, fmt.Errorf("%w: missing exponent digits", ErrParseDecimal)
		}

		var n int
		var err error
		expPart, n, err = parseExp(raw[pos:])
		if err != nil {
			return ast.Decimal{}, err
		}
		pos += n

		if pos != len(raw) {
			return ast.Decimal{}, fmt.Errorf("%w: unexpected %c", ErrParseDecimal, raw[pos])
		}
	}

	digitLen := intEnd + fracLen

	if fracLen != 0 {
		// Remove the decimal point in-place.
		//
		//     10.123
		//       ^
		//     10123
		copy(raw[intEnd:], raw[fracStart:fracEnd])
	}

	digits := raw[:digitLen]

	// Remove leading zeros. This does not change the exponent.
	//
	//     0001e-4 -> 1e-4
	start := 0
	for start < len(digits)-1 && digits[start] == '0' {
		start++
	}
	digits = digits[start:]

	// Zero has one canonical representation regardless of spelling
	// or exponent.
	if len(digits) == 1 && digits[0] == '0' {
		return ast.Decimal{
			Digits: digits,
		}, nil
	}

	// Remove trailing coefficient zeros. Each removed zero increases
	// the exponent by one.
	//
	// Do this before combining with expPart. For example:
	//
	//     1.000e-1000
	//
	// initially has coefficient 1000 and fractional scale -3.
	// Removing the three trailing zeros cancels that -3 exactly,
	// leaving coefficient 1 and exponent -1000.
	trailing := 0
	for len(digits) > 1 && digits[len(digits)-1] == '0' {
		digits = digits[:len(digits)-1]
		trailing++
	}

	// The decimal point contributes -fracLen and canonicalization
	// contributes +trailing:
	//
	//     exp = expPart - fracLen + trailing
	//
	// Compute their difference first. trailing <= frac/int digit count,
	// and using this net adjustment avoids temporary exponent overflow
	// in cases such as 1.0e-9223372036854775808.
	adjust := int64(trailing) - int64(fracLen)

	exp, ok := addInt64(expPart, adjust)
	if !ok {
		return ast.Decimal{}, fmt.Errorf(
			"%w: exponent overflow",
			ErrParseDecimal,
		)
	}

	return ast.Decimal{
		Neg:    neg,
		Digits: digits,
		Exp:    exp,
	}, nil
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

func addInt64(a, b int64) (int64, bool) {
	if b > 0 {
		if a > math.MaxInt64-b {
			return 0, false
		}
	} else if b < 0 {
		if a < math.MinInt64-b {
			return 0, false
		}
	}
	return a + b, true
}

func parseExp(raw []byte) (int64, int, error) {
	if len(raw) == 0 {
		return 0, 0, fmt.Errorf("%w: missing exponent digits", ErrParseDecimal)
	}

	neg := false
	pos := 0

	switch raw[0] {
	case '-':
		neg = true
		fallthrough
	case '+':
		pos++
	}

	if pos == len(raw) || !isDigit(raw[pos]) {
		return 0, 0, fmt.Errorf("%w: missing exponent digits", ErrParseDecimal)
	}

	limit := uint64(math.MaxInt64)
	if neg {
		// abs(MinInt64) == MaxInt64 + 1.
		limit++
	}

	var value uint64
	for pos < len(raw) && isDigit(raw[pos]) {
		digit := uint64(raw[pos] - '0')

		if value > (limit-digit)/10 {
			return 0, 0, fmt.Errorf("%w: exponent overflow", ErrParseDecimal)
		}

		value = value*10 + digit
		pos++
	}

	if neg {
		if value == uint64(math.MaxInt64)+1 {
			return math.MinInt64, pos, nil
		}
		return -int64(value), pos, nil
	}

	return int64(value), pos, nil
}
