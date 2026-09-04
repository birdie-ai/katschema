package decimal_test

import (
	"errors"
	"math"
	"strconv"
	"testing"

	"github.com/birdie-ai/katschema/math/decimal"
	"github.com/google/go-cmp/cmp"
)

func TestParseDecimal(t *testing.T) {
	t.Parallel()

	type testcase struct {
		name string
		text string
		want decimal.Number
		err  error
	}
	for _, tc := range []testcase{
		{
			name: "empty input",
			err:  decimal.ErrParseDecimal,
		},
		{
			text: "+",
			err:  decimal.ErrParseDecimal,
		},
		{
			text: "-",
			err:  decimal.ErrParseDecimal,
		},
		{
			text: ".1",
			err:  decimal.ErrParseDecimal,
		},
		{
			text: "1.",
			err:  decimal.ErrParseDecimal,
		},
		{
			text: "1..1",
			err:  decimal.ErrParseDecimal,
		},
		{
			name: "leading zero in significand is malformed",
			text: "01",
			err:  decimal.ErrParseDecimal,
		},
		{
			name: "leading zero in decimal is malformed",
			text: "00.1",
			err:  decimal.ErrParseDecimal,
		},
		{
			name: "malformed exponent",
			text: "1e",
			err:  decimal.ErrParseDecimalExpMissing,
		},
		{
			text: "1e+",
			err:  decimal.ErrParseDecimalExpMissing,
		},
		{
			text: "1e-",
			err:  decimal.ErrParseDecimalExpMissing,
		},
		{
			text: "1.0e",
			err:  decimal.ErrParseDecimalExpMissing,
		},
		{
			text: "1x",
			err:  decimal.ErrParseDecimal,
		},
		{
			text: "1.2x",
			err:  decimal.ErrParseDecimal,
		},
		{
			text: "1e1x",
			err:  decimal.ErrParseDecimal,
		},
		{
			name: "exponent overflows",
			text: "1e9223372036854775808",
			err:  decimal.ErrParseDecimalExpOverflow,
		},
		{
			name: "exponent + adjust overflows",
			text: "0.1e9223372036854775809",
			err:  decimal.ErrParseDecimalExpOverflow,
		},
		{
			text: "1e-9223372036854775809",
			err:  decimal.ErrParseDecimalExpOverflow,
		},
		{
			name: "canonical exponent is not representable in int64",
			text: "10e9223372036854775807",
			err:  decimal.ErrParseDecimalExpOverflow,
		},
		{
			text: "1.01e-9223372036854775808",
			err:  decimal.ErrParseDecimalExpOverflow,
		},
		{
			text: "1e-9223372036854775809",
			err:  decimal.ErrParseDecimalExpOverflow,
		},
		{
			text: "0.1e-9223372036854775809",
			err:  decimal.ErrParseDecimalExpOverflow,
		},
		{
			text: "+0",
			err:  decimal.ErrParseDecimal,
		},
		{
			text: "0",
			want: decimal.Number{Digits: []byte("0")},
		},
		{
			text: "-0",
			want: decimal.Number{Digits: []byte("0")},
		},

		{
			text: "0.0",
			want: decimal.Number{Digits: []byte("0")},
		},
		{
			text: "-0.000",
			want: decimal.Number{Digits: []byte("0")},
		},
		{
			text: "0e100",
			want: decimal.Number{Digits: []byte("0")},
		},
		{
			text: "-0.000e-1000",
			want: decimal.Number{Digits: []byte("0")},
		},

		{
			text: "1",
			want: decimal.Number{Digits: []byte("1")},
		},
		{
			text: "-1",
			want: decimal.Number{Neg: true, Digits: []byte("1")},
		},
		{
			text: "1.0",
			want: decimal.Number{Digits: []byte("1")},
		},
		{
			text: "1.00",
			want: decimal.Number{Digits: []byte("1")},
		},
		{
			text: "1.0001",
			want: decimal.Number{Digits: []byte("10001"), Exp: -4},
		},
		{
			text: "0.0001",
			want: decimal.Number{Digits: []byte("1"), Exp: -4},
		},
		{
			text: "0.0100",
			want: decimal.Number{Digits: []byte("1"), Exp: -2},
		},
		{
			text: "10.10",
			want: decimal.Number{Digits: []byte("101"), Exp: -1},
		},
		{
			text: "100.000",
			want: decimal.Number{Digits: []byte("1"), Exp: 2},
		},
		{
			text: "123.456",
			want: decimal.Number{Digits: []byte("123456"), Exp: -3},
		},

		{
			text: "1e1",
			want: decimal.Number{Digits: []byte("1"), Exp: 1},
		},
		{
			text: "1e+1",
			want: decimal.Number{Digits: []byte("1"), Exp: 1},
		},
		{
			text: "1e-1",
			want: decimal.Number{Digits: []byte("1"), Exp: -1},
		},
		{
			text: "1E10",
			want: decimal.Number{Digits: []byte("1"), Exp: 10},
		},
		{
			text: "1e01",
			want: decimal.Number{Digits: []byte("1"), Exp: 1},
		},
		{
			text: "1e-001",
			want: decimal.Number{Digits: []byte("1"), Exp: -1},
		},

		{
			text: "1.000e-1",
			want: decimal.Number{Digits: []byte("1"), Exp: -1},
		},
		{
			text: "1.000e-1000",
			want: decimal.Number{Digits: []byte("1"), Exp: -1000},
		},
		{
			text: "1.0e1",
			want: decimal.Number{Digits: []byte("1"), Exp: 1},
		},
		{
			text: "10.0e1",
			want: decimal.Number{Digits: []byte("1"), Exp: 2},
		},
		{
			text: "10.10e2",
			want: decimal.Number{Digits: []byte("101"), Exp: 1},
		},
		{
			text: "0.00100e5",
			want: decimal.Number{Digits: []byte("1"), Exp: 2},
		},
		{
			text: "1000e-3",
			want: decimal.Number{Digits: []byte("1")},
		},
		{
			text: "1000.000e-1000",
			want: decimal.Number{Digits: []byte("1"), Exp: -997},
		},

		{
			text: "9223372036854775807",
			want: decimal.Number{Digits: []byte("9223372036854775807")},
		},
		{
			text: "9223372036854775808",
			want: decimal.Number{Digits: []byte("9223372036854775808")},
		},
		{
			text: "18446744073709551615",
			want: decimal.Number{Digits: []byte("18446744073709551615")},
		},
		{
			text: "123456789012345678901234567890.12345678901234567890",
			want: decimal.Number{
				Digits: []byte("1234567890123456789012345678901234567890123456789"),
				Exp:    -19,
			},
		},

		{
			text: "1e9223372036854775807",
			want: decimal.Number{
				Digits: []byte("1"),
				Exp:    math.MaxInt64,
			},
		},
		{
			text: "1e-9223372036854775808",
			want: decimal.Number{
				Digits: []byte("1"),
				Exp:    math.MinInt64,
			},
		},
		{
			text: "1.0e-9223372036854775808",
			want: decimal.Number{
				Digits: []byte("1"),
				Exp:    math.MinInt64,
			},
		},
		{
			text: "1e9223372036854775807",
			want: decimal.Number{
				Digits: []byte("1"),
				Exp:    9223372036854775807,
			},
		},
		{
			text: "0.1e9223372036854775808",
			want: decimal.Number{
				Digits: []byte("1"),
				Exp:    9223372036854775807,
			},
		},
		{
			text: "0.01e9223372036854775808",
			want: decimal.Number{
				Digits: []byte("1"),
				Exp:    9223372036854775806,
			},
		},
		{
			text: "0.010e9223372036854775808",
			want: decimal.Number{
				Digits: []byte("1"),
				Exp:    9223372036854775806,
			},
		},
		{
			text: "0.0101e9223372036854775808",
			want: decimal.Number{
				Digits: []byte("101"),
				Exp:    9223372036854775804,
			},
		},
	} {
		name := tc.name
		if name == "" {
			name = tc.text
		}
		t.Run(name, func(t *testing.T) {
			got, err := decimal.Parse([]byte(tc.text), []byte(tc.text))
			if !errors.Is(err, tc.err) {
				t.Fatalf("errors mismatch: got[%v] != want[%v]", err, tc.err)
			}
			if tc.err != nil {
				return
			}
			if diff := cmp.Diff(got, tc.want); diff != "" {
				t.Fatalf("decimals differ: got [-], want [+]: %s", diff)
			}
		})
	}
}

var benchmarkDecimal decimal.Number

func BenchmarkParseDecimal(b *testing.B) {
	type testcase struct {
		name string
		text string
	}

	for _, tc := range []testcase{
		{
			name: "integer",
			text: "12345",
		},
		{
			name: "fraction",
			text: "10.10",
		},
		{
			name: "leading_fraction_zeros",
			text: "0.0001",
		},
		{
			name: "trailing_fraction_zeros",
			text: "1.0000",
		},
		{
			name: "exponent",
			text: "1e10",
		},
		{
			name: "fraction_exponent",
			text: "1.000e-1000",
		},
		{
			name: "large_integer",
			text: "1234567890123456789012345678901234567890",
		},
		{
			name: "large_decimal",
			text: "123456789012345678901234567890.12345678901234567890e-1000",
		},
	} {
		b.Run(tc.name, func(b *testing.B) {
			src := []byte(tc.text)

			// Decimal mutates its input. Allocate the working buffer once,
			// then restore it before every parse.
			buf := make([]byte, len(src))

			b.ReportAllocs()
			b.SetBytes(int64(len(src)))

			b.ResetTimer()
			for b.Loop() {
				copy(buf, src)

				v, err := decimal.Parse(buf, buf[:0])
				if err != nil {
					b.Fatal(err)
				}

				benchmarkDecimal = v
			}
		})
	}
}

func BenchmarkParseDecimalIntegerFastPath(b *testing.B) {
	raw := []byte("12345")

	b.ReportAllocs()
	b.SetBytes(int64(len(raw)))
	b.ResetTimer()
	for b.Loop() {
		v, err := decimal.Parse(raw, raw[:0])
		if err != nil {
			b.Fatal(err)
		}
		benchmarkDecimal = v
	}
}

func BenchmarkParseDecimalLong(b *testing.B) {
	for _, size := range []int{1024, 4096} {
		b.Run(strconv.Itoa(size), func(b *testing.B) {
			src := make([]byte, size)

			src[0] = '1'
			for i := 1; i < len(src); i++ {
				src[i] = '2'
			}

			buf := make([]byte, len(src))

			b.ReportAllocs()
			b.SetBytes(int64(len(src)))
			b.ResetTimer()
			for b.Loop() {
				copy(buf, src)

				v, err := decimal.Parse(buf, buf[:0])
				if err != nil {
					b.Fatal(err)
				}
				benchmarkDecimal = v
			}
		})
	}
}

func BenchmarkParseDecimalTrailingZeros(b *testing.B) {
	src := []byte("1.00000000000000000000000000000000000000000000000000")
	buf := make([]byte, len(src))

	b.ReportAllocs()
	b.SetBytes(int64(len(src)))

	for b.Loop() {
		copy(buf, src)

		v, err := decimal.Parse(buf, buf[:0])
		if err != nil {
			b.Fatal(err)
		}
		benchmarkDecimal = v
	}
}
