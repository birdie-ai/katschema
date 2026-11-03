package compiled

import (
	"math"
	"math/big"
	"testing"

	"github.com/birdie-ai/katschema/ks"
)

func TestValidation(t *testing.T) {
	t.Parallel()

	type testcase struct {
		name   string
		schema string
		value  string
		want   bool
	}

	ranged := "(int, 0 <= x <= 100)"
	bigIntRange := "(int, 0 <= x <= " + google + ")"

	for _, tc := range []testcase{
		{
			name:   "string: literal different value",
			schema: `"expected exact string"`,
			value:  `"different string"`,
			want:   false,
		},
		{
			name:   "string: literal exact value",
			schema: `"expected exact string"`,
			value:  `"expected exact string"`,
			want:   true,
		},
		{
			name:   "int: literal exact value",
			schema: "1337",
			value:  "1337",
			want:   true,
		},
		{
			name:   "int: literal diferent value",
			schema: "1337",
			value:  "1338",
			want:   false,
		},
		{
			name:   "bool: literal exact value",
			schema: "true",
			value:  "true",
			want:   true,
		},
		{
			name:   "bool: literal different value",
			schema: "true",
			value:  "false",
			want:   false,
		},
		{
			name:   "null: literal different value",
			schema: "null",
			value:  "true",
			want:   false,
		},
		{
			name:   "null: literal exact value",
			schema: "null",
			value:  "null",
			want:   true,
		},
		{
			name:   "list: literal exact values",
			schema: `["a"]`,
			value:  `["a"]`,
			want:   true,
		},
		{
			name:   "list: literal different values",
			schema: `["a"]`,
			value:  `["b"]`,
			want:   false,
		},
		{
			name:   "list: literal type does not accept empty list",
			schema: `["a"]`,
			value:  "[]",
			want:   false,
		},
		{
			name:   "different type",
			schema: "(string)",
			value:  "10",
			want:   false,
		},
		{
			name:   "(int) against literal int",
			schema: "(int)",
			value:  "67",
			want:   true,
		},
		{
			name:   "(int) rejects literal decimal",
			schema: "(int)",
			value:  "69.0",
			want:   false,
		},
		{
			name:   "(real) accepts a float64 value",
			schema: "(real)",
			value:  "(float64, x == 0.1)",
			want:   true,
		},
		{
			name:   "(real) accepts a float32 value",
			schema: "(real)",
			value:  "(float32, x == 0.1)",
			want:   true,
		},
		{
			name:   "(real) compares a float64 value exactly",
			schema: "(real, x >= 0.1)",
			value:  "(float64, x == 0.1)",
			want:   true,
		},
		{
			name:   "(real) equate float64 0.1 with exact decimal 0.1",
			schema: "(real, x == 0.1)",
			value:  "(float64, x == 0.1)",
			want:   true,
		},
		{
			name:   "float32 accepts an exact float32 value",
			schema: "(float32)",
			value:  "(float32, x == 0.1)",
			want:   true,
		},
		{
			name:   "float32 accepts a float64 value rounded by conversion",
			schema: "(float32)",
			value:  "(float64, x == 0.1)",
			want:   true,
		},
		{
			name:   "float32 rejects a value outside its finite range",
			schema: "(float32)",
			value:  "(float64, x == 1e39)",
			want:   false,
		},
		{
			name:   "float64 accepts a float64 value",
			schema: "(float64)",
			value:  "(float64, x == 0.1)",
			want:   true,
		},
		{
			name:   "float64 accepts a float32 value",
			schema: "(float64)",
			value:  "(float32, x == 0.1)",
			want:   true,
		},
		{
			name:   "float64 rejects a value outside its finite range",
			schema: "(float64)",
			value:  "1e309",
			want:   false,
		},

		{
			name:   "(float) accepts integers converted to float",
			schema: "(float)",
			value:  "18014398509481984",
			want:   true,
		},
		{
			name:   "(float) accepts integers rounded during conversion",
			schema: "(float)",
			value:  "18014398509481985",
			want:   true,
		},

		{
			name:   "valid on the lower bound edge",
			schema: ranged,
			value:  "0",
			want:   true,
		},
		{
			name:   "valid on the upper bound edge",
			schema: ranged,
			value:  "100",
			want:   true,
		},
		{
			name:   "valid somewhere in the middle",
			schema: ranged,
			value:  "67",
			want:   true,
		},
		{
			name:   "outside range is rejected",
			schema: ranged,
			value:  "1337",
			want:   false,
		},
		{
			name:   "bigInt outside range is rejected",
			schema: ranged,
			value:  google,
			want:   false,
		},
		{
			name:   "valid upper bound in bigInt range",
			schema: bigIntRange,
			value:  google,
			want:   true,
		},
		{
			name:   "valid value inside int8",
			schema: "(int8)",
			value:  "100",
			want:   true,
		},
		{
			name:   "math.MinInt8 is valid int8",
			schema: "(int8)",
			value:  "-128",
			want:   true,
		},
		{
			name:   "math.MaxInt8 is valid int8",
			schema: "(int8)",
			value:  "127",
			want:   true,
		},
		{
			name:   "math.MaxInt8+1 is outside int8 bounds",
			schema: "(int8)",
			value:  "128",
			want:   false,
		},
		{
			name:   "math.MinInt8-1 is outside int8 bounds",
			schema: "(int8)",
			value:  "-129",
			want:   false,
		},
		{
			name:   "int: value has valid in enum",
			schema: "(int, x in [0, 1, 2])",
			value:  "1",
			want:   true,
		},
		{
			name:   "int: value has valid not in enum",
			schema: "(int, x in [0, 1, 2])",
			value:  "3",
			want:   false,
		},
		{
			name:   "required field with provided value",
			schema: `{"a": (string)}`,
			value:  `{"a": "xxx"}`,
			want:   true,
		},
		{
			name:   "required field with absent value",
			schema: `{"a": (string)}`,
			value:  "{}",
			want:   false,
		},
		{
			name:   "optional field missing is fine",
			schema: `{"a": (string), "b": (int, optional)}`,
			value:  `{"a": "Triple X"}`,
			want:   true,
		},
		{
			name:   "optional field is an object with required fields",
			schema: `{"a": (string), "b": ({"a": (string), "b": (int)}, optional)}`,
			value:  `{"a": "Triple X"}`,
			want:   true,
		},
		{
			name:   "value has extra field",
			schema: `{"a": (string)}`,
			value:  `{"a": "xxx", "b": "yyy"}`,
			want:   false,
		},
		{
			name:   "validation recurses in object fields : missing b.b",
			schema: `{"a": (string), "b": ({"a": (string), "b": (int)}, optional)}`,
			value:  `{"a": "xXx", "b": {"a": "Xxx"}}`,
			want:   false,
		},
		{
			name:   "validation recurses in object fields : all keys provided",
			schema: `{"a": (string), "b": ({"a": (string), "b": (int)}, optional)}`,
			value:  `{"a": "xXx", "b": {"a": "Xxx", "b": 1337}}`,
			want:   true,
		},
		{
			name:   "[(string)] accepts []",
			schema: "[(string)]",
			value:  "[]",
			want:   true,
		},
		{
			name:   "[(string)] accepts [... strings ...]",
			schema: "[(string)]",
			value:  `["kat", "schema"]`,
			want:   true,
		},
		{
			name:   "[(string)] rejects [... integers ...]",
			schema: "[(string)]",
			value:  "[67, 69]",
			want:   false,
		},
		{
			name:   "[(string)] rejects [... mixed types ...]",
			schema: "[(string)]",
			value:  `["valid", 67]`,
			want:   false,
		},
		{
			name:   "[(int, x > 0)] accepts []",
			schema: "[(int, x > 0)]",
			value:  "[]",
			want:   true,
		},
		{
			name:   "[(int, x > 0)] accepts [1, 2, 3]",
			schema: "[(int, x > 0)]",
			value:  "[1, 2, 3]",
			want:   true,
		},
		{
			name:   "[(int, x > 0)] rejects [1, 2, 3, -1]",
			schema: "[(int, x > 0)]",
			value:  "[1, 2, 3, -1]",
			want:   false,
		},
		{
			name:   "([(int)], len(x)>2) accepts [1, 2, 3]",
			schema: "([(int)], len(x) > 2)",
			value:  "[1, 2, 3]",
			want:   true,
		},
		{
			name:   "([(int)], len(x)>2) rejects [1, 2]",
			schema: "([(int)], len(x) > 2)",
			value:  "[1, 2]",
			want:   false,
		},
		{
			name:   "([(int, x < 0)], len(x)>2) accepts [-1, -2, -3]",
			schema: "([(int, x < 0)], len(x) > 2)",
			value:  "[-1, -2, -3]",
			want:   true,
		},
		{
			name:   "([(int, x < 0)], len(x)>2) rejects [-1, -2, 1]",
			schema: "([(int, x < 0)], len(x) > 2)",
			value:  "[-1, -2, 1]",
			want:   false,
		},
		{
			name:   "([(int)], len(x)>2) rejects [1, 2]",
			schema: "([(int)], len(x) > 2)",
			value:  "[1, 2]",
			want:   false,
		},
		{
			name:   "[(int, x > 0)] rejects [1, 2, 3, -1]",
			schema: "[(int, x > 0)]",
			value:  "[1, 2, 3, -1]",
			want:   false,
		},
		{
			name:   "[(int)]: value has valid in enum",
			schema: "[(int, x in [0, 1, 2])]",
			value:  "[1]",
			want:   true,
		},
		{
			name:   "[(int)]: value not in enum",
			schema: "[(int, x in [0, 1, 2])]",
			value:  "[3]",
			want:   false,
		},
		{
			name:   "(real): check if endpoint is excluded",
			schema: "(real, x > 0.1)",
			value:  "0.1",
		},
		{
			name:   "(real): arbitrarily close value is accepted",
			schema: "(real, x > 0.1)",
			value:  "(real, x == 0.1000000000000000000000001)",
			want:   true,
		},
		{
			name:   "(real): integer values are accepted as real values",
			schema: "(real, x > 0.1)",
			value:  "10",
			want:   true,
		},
		{
			name:   "(real): value below bound is rejected",
			schema: "(real, x > 0.1)",
			value:  "0.099999999999999999999999999999999999999999999999999999999999999",
			want:   false,
		},
		{
			name:   "(real): number inside of arbitrarily large constraint bound (edge)",
			schema: "(real, x >= 1234567890123456789012345678901234567890.00000000000000000001)",
			value:  "1234567890123456789012345678901234567890.00000000000000000001",
			want:   true,
		},
		{
			name:   "(real): number one unit below arbitrarily large bound",
			schema: "(real, x >= 1234567890123456789012345678901234567890.00000000000000000001)",
			value:  "1234567890123456789012345678901234567890.00000000000000000000",
		},
		{
			name:   "(real): number with large exponent within the bound",
			schema: "(real, x >= 1234567890123456789012345678901234567890.00000000000000000001)",
			value:  "2e1000",
			want:   true,
		},
		{
			name:   "(real): check enum + bound using int",
			schema: "(real, x > 1.0, x in [1, 2.0])",
			value:  "1",
			want:   false,
		},
		{
			name:   "(real): check enum honour strict bound using real",
			schema: "(real, x > 1.0, x in [1, 2.0])",
			value:  "2.0",
			want:   true,
		},
		{
			name:   "(real): >= accepts the exact lower edge",
			schema: "(real, x >= 0.1)",
			value:  "0.1",
			want:   true,
		},
		{
			name:   "(real): >= rejects values below the lower edge",
			schema: "(real, x >= 0.1)",
			value:  "0.099999999999999999",
		},
		{
			name:   "(real): < rejects the exact upper edge",
			schema: "(real, x < 0.1)",
			value:  "0.1",
		},
		{
			name:   "(real): < accepts values below the upper edge",
			schema: "(real, x < 0.1)",
			value:  "0.099999999999999999",
			want:   true,
		},
		{
			name:   "(real): <= accepts the exact upper edge",
			schema: "(real, x <= 0.1)",
			value:  "0.1",
			want:   true,
		},
		{
			name:   "(real): <= rejects values above the upper edge",
			schema: "(real, x <= 0.1)",
			value:  "0.100000000000000001",
		},
		{
			name:   "(real): reversed lower comparison is normalized",
			schema: "(real, 0.1 < x)",
			value:  "0.100000000000000001",
			want:   true,
		},
		{
			name:   "(real): reversed upper comparison is normalized",
			schema: "(real, 0.1 > x)",
			value:  "0.099999999999999999",
			want:   true,
		},
		{
			name:   "(real): stronger lower bound replaces weaker lower bound",
			schema: "(real, x >= 0.1, x >= 0.2)",
			value:  "0.15",
		},
		{
			// NOTE(i4k): this checks internal normalization order.
			name:   "(real): weaker lower bound does not replace stronger lower bound",
			schema: "(real, x >= 0.2, x >= 0.1)",
			value:  "0.15",
		},
		{
			name:   "(real): stronger upper bound replaces weaker upper bound",
			schema: "(real, x <= 0.2, x <= 0.1)",
			value:  "0.15",
		},
		{
			name:   "(real): weaker upper bound does not replace stronger upper bound",
			schema: "(real, x <= 0.1, x <= 0.2)",
			value:  "0.15",
		},
		{
			name:   "(real): strict lower bound replaces inclusive bound at the same edge",
			schema: "(real, x >= 1.0, x > 1.0)",
			value:  "1.0",
		},
		{
			name:   "(real): inclusive lower bound does not weaken a strict bound at the same edge",
			schema: "(real, x > 1.0, x >= 1.0)",
			value:  "1.0",
		},
		{
			name:   "(real): strict upper bound replaces inclusive bound at the same edge",
			schema: "(real, x <= 1.0, x < 1.0)",
			value:  "1.0",
		},
		{
			name:   "(real): inclusive upper bound does not weaken a strict bound at the same edge",
			schema: "(real, x < 1.0, x <= 1.0)",
			value:  "1.0",
		},
		{
			name:   "(real): equal inclusive bounds accept the singleton edge",
			schema: "(real, x >= 1.0, x <= 1.0)",
			value:  "1.0",
			want:   true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := NewArena()
			typ := compileText(t, a, tc.schema)
			tree, val := parseText(t, tc.value)
			if got := a.Valid(typ, tree, val); got != tc.want {
				t.Fatalf("Valid = %v, want %v", got, tc.want)
			}
			for _, values := range []*Arena{a, NewArena()} {
				value := values.Type(compileText(t, values, tc.value))
				if got := a.Type(typ).ValidType(value); got != tc.want {
					t.Fatalf("ValidType = %v, want %v", got, tc.want)
				}
			}
		})
	}
}

func TestValidIntrinsicIntegerBounds(t *testing.T) {
	t.Parallel()

	type testcase struct {
		schema  ks.Value
		valid   []ks.Value
		invalid []ks.Value
	}

	var googleBigInt big.Int
	_, ok := googleBigInt.SetString(google, 10)
	if !ok {
		t.Fatal("unreachable")
	}

	nextMaxInt64 := big.NewInt(math.MaxInt64)
	nextMaxInt64.Add(nextMaxInt64, big.NewInt(1))

	prevMinInt64 := big.NewInt(math.MinInt64)
	prevMinInt64.Add(prevMinInt64, big.NewInt(-1))

	var maxUint64 big.Int
	maxUint64.SetUint64(uint64(math.MaxUint64))

	a := NewArena()
	for _, tc := range []testcase{
		{
			schema: ks.Int(),
			valid: []ks.Value{
				ks.LitInt(math.MinInt64),
				ks.LitInt(0),
				ks.LitInt(100),
				ks.LitInt(math.MaxInt64),
				ks.LitBigInt(&googleBigInt),
			},
			invalid: []ks.Value{
				ks.LitDecimal("10"),
			},
		},
		{
			schema: ks.Int8(),
			valid: []ks.Value{
				ks.LitInt(math.MinInt8),
				ks.LitInt(0),
				ks.LitInt(100),
				ks.LitInt(math.MaxInt8),
			},
			invalid: []ks.Value{
				ks.LitInt(math.MinInt8 - 1),
				ks.LitInt(math.MaxInt8 + 1),
				ks.LitInt(math.MinInt32),
				ks.LitInt(math.MaxInt32),
				ks.LitInt(math.MinInt64),
				ks.LitInt(math.MaxInt64),
				ks.LitBigInt(&googleBigInt),
				ks.LitDecimal("10"),
			},
		},
		{
			schema: ks.Int16(),
			valid: []ks.Value{
				ks.LitInt(math.MinInt16),
				ks.LitInt(0),
				ks.LitInt(100),
				ks.LitInt(math.MaxInt16),
			},
			invalid: []ks.Value{
				ks.LitInt(math.MinInt16 - 1),
				ks.LitInt(math.MaxInt16 + 1),
				ks.LitInt(math.MinInt32),
				ks.LitInt(math.MaxInt32),
				ks.LitInt(math.MinInt64),
				ks.LitInt(math.MaxInt64),
				ks.LitBigInt(&googleBigInt),
				ks.LitDecimal("10"),
			},
		},
		{
			schema: ks.Int32(),
			valid: []ks.Value{
				ks.LitInt(math.MinInt32),
				ks.LitInt(0),
				ks.LitInt(100),
				ks.LitInt(math.MaxInt32),
			},
			invalid: []ks.Value{
				ks.LitInt(math.MinInt32 - 1),
				ks.LitInt(math.MaxInt32 + 1),
				ks.LitInt(math.MinInt64),
				ks.LitInt(math.MaxInt64),
				ks.LitBigInt(&googleBigInt),
				ks.LitDecimal("10"),
			},
		},
		{
			schema: ks.Int64(),
			valid: []ks.Value{
				ks.LitInt(math.MinInt64),
				ks.LitInt(0),
				ks.LitInt(100),
				ks.LitInt(math.MaxInt64),
			},
			invalid: []ks.Value{
				ks.LitBigInt(nextMaxInt64),
				ks.LitBigInt(prevMinInt64),
				ks.LitBigInt(&googleBigInt),
				ks.LitDecimal("10"),
			},
		},
		{
			schema: ks.Uint8(),
			valid: []ks.Value{
				ks.LitInt(0),
				ks.LitInt(100),
				ks.LitInt(math.MaxUint8),
			},
			invalid: []ks.Value{
				ks.LitInt(-1),
				ks.LitInt(math.MaxUint8 + 1),
				ks.LitInt(math.MinInt32),
				ks.LitInt(math.MaxInt32),
				ks.LitInt(math.MinInt64),
				ks.LitInt(math.MaxInt64),
				ks.LitBigInt(&googleBigInt),
				ks.LitDecimal("10"),
			},
		},
		{
			schema: ks.Uint16(),
			valid: []ks.Value{
				ks.LitInt(0),
				ks.LitInt(100),
				ks.LitInt(math.MaxUint16),
			},
			invalid: []ks.Value{
				ks.LitInt(-1),
				ks.LitInt(math.MaxUint16 + 1),
				ks.LitInt(math.MinInt32),
				ks.LitInt(math.MaxInt32),
				ks.LitInt(math.MinInt64),
				ks.LitInt(math.MaxInt64),
				ks.LitBigInt(&googleBigInt),
				ks.LitDecimal("10"),
			},
		},
		{
			schema: ks.Uint32(),
			valid: []ks.Value{
				ks.LitInt(0),
				ks.LitInt(100),
				ks.LitInt(math.MaxUint32),
			},
			invalid: []ks.Value{
				ks.LitInt(-1),
				ks.LitInt(math.MaxUint32 + 1),
				ks.LitInt(math.MinInt64),
				ks.LitInt(math.MaxInt64),
				ks.LitBigInt(&googleBigInt),
				ks.LitDecimal("10"),
			},
		},
		{
			schema: ks.Uint64(),
			valid: []ks.Value{
				ks.LitInt(0),
				ks.LitInt(100),
				ks.LitBigInt(nextMaxInt64),
				ks.LitBigInt(&maxUint64),
			},
			invalid: []ks.Value{
				ks.LitInt(-1),
				ks.LitBigInt(&googleBigInt),
				ks.LitDecimal("10"),
			},
		},
	} {
		typ := compile(t, a, tc.schema)
		for _, v := range tc.valid {
			tree, val := build(t, v)
			if ok := a.Valid(typ, tree, val); !ok {
				t.Fatalf("Valid = %v, want true", ok)
			}
		}
		for _, v := range tc.invalid {
			tree, val := build(t, v)
			if ok := a.Valid(typ, tree, val); ok {
				t.Fatalf("Valid = %v, want false", ok)
			}
		}
	}
}
