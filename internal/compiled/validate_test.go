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
		schema ks.Value
		value  ks.Value
		want   bool
	}

	// (int, 0 <= x <= 100)
	ranged := ks.Where(ks.Int(), ks.Binary(
		ks.Binary(ks.IntExpr(0), ks.Le, ks.X()),
		ks.Le,
		ks.IntExpr(100),
	))

	var googleBigInt big.Int
	_, ok := googleBigInt.SetString(google, 10)
	if !ok {
		t.Fatal("failed to encode google bigInt")
	}

	var nextGoogleBigInt big.Int
	nextGoogleBigInt.Add(&googleBigInt, big.NewInt(1))

	// (int, 0 <= x <= 10^100)
	bigIntRange := ks.Where(ks.Int(), ks.Binary(
		ks.Binary(ks.IntExpr(0), ks.Le, ks.X()),
		ks.Le,
		ks.ValueExpr(ks.LitBigInt(&googleBigInt)),
	))

	for _, tc := range []testcase{
		{
			name:   "string: literal different value",
			schema: ks.LitString("expected exact string"),
			value:  ks.LitString("different string"),
			want:   false,
		},
		{
			name:   "string: literal exact value",
			schema: ks.LitString("expected exact string"),
			value:  ks.LitString("expected exact string"),
			want:   true,
		},
		{
			name:   "int: literal exact value",
			schema: ks.LitInt(1337),
			value:  ks.LitInt(1337),
			want:   true,
		},
		{
			name:   "int: literal diferent value",
			schema: ks.LitInt(1337),
			value:  ks.LitInt(1338),
			want:   false,
		},
		{
			name:   "bool: literal exact value",
			schema: ks.LitBool(true),
			value:  ks.LitBool(true),
			want:   true,
		},
		{
			name:   "bool: literal different value",
			schema: ks.LitBool(true),
			value:  ks.LitBool(false),
			want:   false,
		},
		{
			name:   "null: literal different value",
			schema: ks.LitNull(),
			value:  ks.LitBool(true),
			want:   false,
		},
		{
			name:   "null: literal exact value",
			schema: ks.LitNull(),
			value:  ks.LitNull(),
			want:   true,
		},
		{
			name:   "list: literal exact values",
			schema: ks.List(ks.LitString("a")),
			value:  ks.List(ks.LitString("a")),
			want:   true,
		},
		{
			name:   "list: literal different values",
			schema: ks.List(ks.LitString("a")),
			value:  ks.List(ks.LitString("b")),
			want:   false,
		},
		{
			name:   "list: literal type does not accept empty list",
			schema: ks.List(ks.LitString("a")),
			value:  ks.List(),
			want:   false,
		},
		{
			name:   "different type",
			schema: ks.String(),
			value:  ks.LitInt(10),
			want:   false,
		},
		{
			name:   "(int) against literal int",
			schema: ks.Int(),
			value:  ks.LitInt(67),
			want:   true,
		},
		{
			name:   "(int) rejects literal float",
			schema: ks.Int(),
			value:  ks.LitFloat(69.0),
			want:   false,
		},

		// NOTE(i4k): BETTER SAFE THAN SORRY!
		// Finally solving that subtle bug when use large integers in JSON :-)
		{

			name:   "(float) accepts integers that can be representable in float without loss of precision",
			schema: ks.Float(),
			value:  ks.LitInt(int64(2<<53) + 0),
			want:   true,
		},
		{

			name:   "(float) rejects integers that cannot be representable in float",
			schema: ks.Float(),
			value:  ks.LitInt(int64(2<<53) + 1),
			want:   false,
		},

		{
			name:   "valid on the lower bound edge",
			schema: ranged,
			value:  ks.LitInt(0),
			want:   true,
		},
		{
			name:   "valid on the upper bound edge",
			schema: ranged,
			value:  ks.LitInt(100),
			want:   true,
		},
		{
			name:   "valid somewhere in the middle",
			schema: ranged,
			value:  ks.LitInt(67),
			want:   true,
		},
		{
			name:   "outside range is rejected",
			schema: ranged,
			value:  ks.LitInt(1337),
			want:   false,
		},
		{
			name:   "bigInt outside range is rejected",
			schema: ranged,
			value:  ks.LitBigInt(&googleBigInt),
			want:   false,
		},
		{
			name:   "valid upper bound in bigInt range",
			schema: bigIntRange,
			value:  ks.LitBigInt(&googleBigInt),
			want:   true,
		},
		{
			name:   "valid value inside int8",
			schema: ks.Int8(),
			value:  ks.LitInt(100),
			want:   true,
		},
		{
			name:   "math.MinInt8 is valid int8",
			schema: ks.Int8(),
			value:  ks.LitInt(math.MinInt8),
			want:   true,
		},
		{
			name:   "math.MaxInt8 is valid int8",
			schema: ks.Int8(),
			value:  ks.LitInt(math.MaxInt8),
			want:   true,
		},
		{
			name:   "math.MaxInt8+1 is outside int8 bounds",
			schema: ks.Int8(),
			value:  ks.LitInt(math.MaxInt8 + 1),
			want:   false,
		},
		{
			name:   "math.MinInt8-1 is outside int8 bounds",
			schema: ks.Int8(),
			value:  ks.LitInt(math.MinInt8 - 1),
			want:   false,
		},
		{
			name: "int: value has valid in enum",
			schema: ks.With(
				ks.Int(),
				ks.Check(
					ks.Binary(
						ks.X(),
						ks.In,
						ks.ListExpr(ks.LitInt(0), ks.LitInt(1), ks.LitInt(2)),
					),
				),
			),
			value: ks.LitInt(1),
			want:  true,
		},
		{
			name: "int: value has valid not in enum",
			schema: ks.With(
				ks.Int(),
				ks.Check(
					ks.Binary(
						ks.X(),
						ks.In,
						ks.ListExpr(ks.LitInt(0), ks.LitInt(1), ks.LitInt(2)),
					),
				),
			),
			value: ks.LitInt(3),
			want:  false,
		},
		{
			name:   "required field with provided value",
			schema: ks.Object(ks.Field("a", ks.String())),
			value:  ks.Object(ks.Field("a", ks.LitString("xxx"))),
			want:   true,
		},
		{
			name:   "required field with absent value",
			schema: ks.Object(ks.Field("a", ks.String())),
			value:  ks.Object(),
			want:   false,
		},
		{
			name: "optional field missing is fine",
			schema: ks.Object(
				ks.Field("a", ks.String()),
				ks.Field("b", ks.Optional(ks.Int())),
			),
			value: ks.Object(ks.Field("a", ks.LitString("Triple X"))),
			want:  true,
		},
		{
			name: "optional field is an object with required fields",
			schema: ks.Object(
				ks.Field("a", ks.String()),
				ks.Field("b", ks.Optional(ks.Object(
					ks.Field("a", ks.String()),
					ks.Field("b", ks.Int()),
				))),
			),
			value: ks.Object(ks.Field("a", ks.LitString("Triple X"))),
			want:  true,
		},
		{
			name:   "value has extra field",
			schema: ks.Object(ks.Field("a", ks.String())),
			value: ks.Object(
				ks.Field("a", ks.LitString("xxx")),
				ks.Field("b", ks.LitString("yyy")),
			),
			want: false,
		},
		{
			name: "validation recurses in object fields : missing b.b",
			schema: ks.Object(
				ks.Field("a", ks.String()),
				ks.Field("b", ks.Optional(ks.Object(
					ks.Field("a", ks.String()),
					ks.Field("b", ks.Int()),
				))),
			),
			value: ks.Object(
				ks.Field("a", ks.LitString("xXx")),
				ks.Field("b", ks.Object(
					ks.Field("a", ks.LitString("Xxx")),
				)),
			),
			want: false,
		},
		{
			name: "validation recurses in object fields : all keys provided",
			schema: ks.Object(
				ks.Field("a", ks.String()),
				ks.Field("b", ks.Optional(ks.Object(
					ks.Field("a", ks.String()),
					ks.Field("b", ks.Int()),
				))),
			),
			value: ks.Object(
				ks.Field("a", ks.LitString("xXx")),
				ks.Field("b", ks.Object(
					ks.Field("a", ks.LitString("Xxx")),
					ks.Field("b", ks.LitInt(1337)),
				)),
			),
			want: true,
		},
		{
			name:   "[(string)] accepts []",
			schema: ks.List(ks.String()),
			value:  ks.List(),
			want:   true,
		},
		{
			name:   "[(string)] accepts [... strings ...]",
			schema: ks.List(ks.String()),
			value:  ks.List(ks.LitString("kat"), ks.LitString("schema")),
			want:   true,
		},
		{
			name:   "[(string)] rejects [... integers ...]",
			schema: ks.List(ks.String()),
			value:  ks.List(ks.LitInt(67), ks.LitInt(69)),
			want:   false,
		},
		{
			name:   "[(string)] rejects [... mixed types ...]",
			schema: ks.List(ks.String()),
			value:  ks.List(ks.LitString("valid"), ks.LitInt(67)),
			want:   false,
		},
		{
			name:   "[(int, x > 0)] accepts []",
			schema: ks.List(ks.With(ks.Int(), ks.Check(ks.Binary(ks.X(), ks.Gt, ks.ValueExpr(ks.LitInt(0)))))),
			value:  ks.List(),
			want:   true,
		},
		{
			name:   "[(int, x > 0)] accepts [1, 2, 3]",
			schema: ks.List(ks.With(ks.Int(), ks.Check(ks.Binary(ks.X(), ks.Gt, ks.ValueExpr(ks.LitInt(0)))))),
			value:  ks.List(ks.LitInt(1), ks.LitInt(2), ks.LitInt(3)),
			want:   true,
		},
		{
			name:   "[(int, x > 0)] rejects [1, 2, 3, -1]",
			schema: ks.List(ks.With(ks.Int(), ks.Check(ks.Binary(ks.X(), ks.Gt, ks.ValueExpr(ks.LitInt(0)))))),
			value:  ks.List(ks.LitInt(1), ks.LitInt(2), ks.LitInt(3), ks.LitInt(-1)),
			want:   false,
		},
		{
			name: "([(int)], len(x)>2) accepts [1, 2, 3]",
			schema: ks.With(
				ks.List(ks.Int()),
				ks.Check(
					ks.Binary(
						ks.Funcall("len", ks.X()),
						ks.Gt,
						ks.ValueExpr(ks.LitInt(2)),
					),
				),
			),
			value: ks.List(ks.LitInt(1), ks.LitInt(2), ks.LitInt(3)),
			want:  true,
		},
		{
			name: "([(int)], len(x)>2) rejects [1, 2]",
			schema: ks.With(
				ks.List(ks.Int()),
				ks.Check(
					ks.Binary(
						ks.Funcall("len", ks.X()),
						ks.Gt,
						ks.ValueExpr(ks.LitInt(2)),
					),
				),
			),
			value: ks.List(ks.LitInt(1), ks.LitInt(2)),
			want:  false,
		},
		{
			name: "([(int, x < 0)], len(x)>2) accepts [-1, -2, -3]",
			schema: ks.With(
				ks.List(
					ks.With(
						ks.Int(),
						ks.Check(
							ks.Binary(ks.X(), ks.Lt, ks.ValueExpr(ks.LitInt(0))),
						),
					),
				),
				ks.Check(
					ks.Binary(
						ks.Funcall("len", ks.X()),
						ks.Gt,
						ks.ValueExpr(ks.LitInt(2)),
					),
				),
			),
			value: ks.List(ks.LitInt(-1), ks.LitInt(-2), ks.LitInt(-3)),
			want:  true,
		},
		{
			name: "([(int, x < 0)], len(x)>2) rejects [-1, -2, 1]",
			schema: ks.With(
				ks.List(
					ks.With(
						ks.Int(),
						ks.Check(
							ks.Binary(ks.X(), ks.Lt, ks.ValueExpr(ks.LitInt(0))),
						),
					),
				),
				ks.Check(
					ks.Binary(
						ks.Funcall("len", ks.X()),
						ks.Gt,
						ks.ValueExpr(ks.LitInt(2)),
					),
				),
			),
			value: ks.List(ks.LitInt(-1), ks.LitInt(-2), ks.LitInt(1)),
			want:  false,
		},
		{
			name: "([(int)], len(x)>2) rejects [1, 2]",
			schema: ks.With(
				ks.List(ks.Int()),
				ks.Check(
					ks.Binary(
						ks.Funcall("len", ks.X()),
						ks.Gt,
						ks.ValueExpr(ks.LitInt(2)),
					),
				),
			),
			value: ks.List(ks.LitInt(1), ks.LitInt(2)),
			want:  false,
		},
		{
			name:   "[(int, x > 0)] rejects [1, 2, 3, -1]",
			schema: ks.List(ks.With(ks.Int(), ks.Check(ks.Binary(ks.X(), ks.Gt, ks.ValueExpr(ks.LitInt(0)))))),
			value:  ks.List(ks.LitInt(1), ks.LitInt(2), ks.LitInt(3), ks.LitInt(-1)),
			want:   false,
		},
		{
			name: "[(int)]: value has valid in enum",
			schema: ks.List(ks.With(
				ks.Int(),
				ks.Check(
					ks.Binary(
						ks.X(),
						ks.In,
						ks.ListExpr(ks.LitInt(0), ks.LitInt(1), ks.LitInt(2)),
					),
				),
			)),
			value: ks.List(ks.LitInt(1)),
			want:  true,
		},
		{
			name: "[(int)]: value not in enum",
			schema: ks.List(ks.With(
				ks.Int(),
				ks.Check(
					ks.Binary(
						ks.X(),
						ks.In,
						ks.ListExpr(ks.LitInt(0), ks.LitInt(1), ks.LitInt(2)),
					),
				),
			)),
			value: ks.List(ks.LitInt(3)),
			want:  false,
		},
		{
			name:   "(real): check if endpoint is excluded",
			schema: ks.Where(ks.Real(), ks.Binary(ks.X(), ks.Gt, ks.RealExpr("0.1"))),
			value:  ks.LitReal("0.1"),
		},
		{
			name:   "(real): arbitrarily close value is accepted",
			schema: ks.Where(ks.Real(), ks.Binary(ks.X(), ks.Gt, ks.RealExpr("0.1"))),
			value:  ks.LitReal("0.1000000000000000000000001"),
			want:   true,
		},
		{
			name:   "(real): integer values are accepted as real values",
			schema: ks.Where(ks.Real(), ks.Binary(ks.X(), ks.Gt, ks.RealExpr("0.1"))),
			value:  ks.LitInt(10),
			want:   true,
		},
		{
			name:   "(real): value below bound is rejected",
			schema: ks.Where(ks.Real(), ks.Binary(ks.X(), ks.Gt, ks.RealExpr("0.1"))),
			value:  ks.LitReal("0.099999999999999999999999999999999999999999999999999999999999999"),
			want:   false,
		},
		{
			name:   "(real): number inside of arbitrarily large constraint bound (edge)",
			schema: ks.Where(ks.Real(), ks.Binary(ks.X(), ks.Ge, ks.RealExpr("1234567890123456789012345678901234567890.00000000000000000001"))),
			value:  ks.LitDecimal("1234567890123456789012345678901234567890.00000000000000000001"),
			want:   true,
		},
		{
			name:   "(real): number one unit below arbitrarily large bound",
			schema: ks.Where(ks.Real(), ks.Binary(ks.X(), ks.Ge, ks.RealExpr("1234567890123456789012345678901234567890.00000000000000000001"))),
			value:  ks.LitDecimal("1234567890123456789012345678901234567890.00000000000000000000"),
		},
		{
			name:   "(real): number with large exponent within the bound",
			schema: ks.Where(ks.Real(), ks.Binary(ks.X(), ks.Ge, ks.RealExpr("1234567890123456789012345678901234567890.00000000000000000001"))),
			value:  ks.LitReal("2e1000"),
			want:   true,
		},
		{
			name: "(real): check enum + bound using int",
			schema: ks.With(ks.Real(),
				ks.Check(ks.Binary(ks.X(), ks.Gt, ks.RealExpr("1"))),
				ks.Check(ks.Binary(ks.X(), ks.In, ks.ListExpr(ks.LitInt(1), ks.LitReal("2")))),
			),
			value: ks.LitInt(1),
			want:  false,
		},
		{
			name: "(real): check enum honour strict bound using real",
			schema: ks.With(ks.Real(),
				ks.Check(ks.Binary(ks.X(), ks.Gt, ks.RealExpr("1"))),
				ks.Check(ks.Binary(ks.X(), ks.In, ks.ListExpr(ks.LitInt(1), ks.LitReal("2.0")))),
			),
			value: ks.LitReal("2"),
			want:  true,
		},
		{
			name:   "(real): >= accepts the exact lower edge",
			schema: ks.Where(ks.Real(), ks.Binary(ks.X(), ks.Ge, ks.RealExpr("0.1"))),
			value:  ks.LitReal("0.1"),
			want:   true,
		},
		{
			name:   "(real): >= rejects values below the lower edge",
			schema: ks.Where(ks.Real(), ks.Binary(ks.X(), ks.Ge, ks.RealExpr("0.1"))),
			value:  ks.LitReal("0.099999999999999999"),
		},
		{
			name:   "(real): < rejects the exact upper edge",
			schema: ks.Where(ks.Real(), ks.Binary(ks.X(), ks.Lt, ks.RealExpr("0.1"))),
			value:  ks.LitReal("0.1"),
		},
		{
			name:   "(real): < accepts values below the upper edge",
			schema: ks.Where(ks.Real(), ks.Binary(ks.X(), ks.Lt, ks.RealExpr("0.1"))),
			value:  ks.LitReal("0.099999999999999999"),
			want:   true,
		},
		{
			name:   "(real): <= accepts the exact upper edge",
			schema: ks.Where(ks.Real(), ks.Binary(ks.X(), ks.Le, ks.RealExpr("0.1"))),
			value:  ks.LitReal("0.1"),
			want:   true,
		},
		{
			name:   "(real): <= rejects values above the upper edge",
			schema: ks.Where(ks.Real(), ks.Binary(ks.X(), ks.Le, ks.RealExpr("0.1"))),
			value:  ks.LitReal("0.100000000000000001"),
		},
		{
			name:   "(real): reversed lower comparison is normalized",
			schema: ks.Where(ks.Real(), ks.Binary(ks.RealExpr("0.1"), ks.Lt, ks.X())),
			value:  ks.LitReal("0.100000000000000001"),
			want:   true,
		},
		{
			name:   "(real): reversed upper comparison is normalized",
			schema: ks.Where(ks.Real(), ks.Binary(ks.RealExpr("0.1"), ks.Gt, ks.X())),
			value:  ks.LitReal("0.099999999999999999"),
			want:   true,
		},
		{
			name: "(real): stronger lower bound replaces weaker lower bound",
			schema: ks.With(ks.Real(),
				ks.Check(ks.Binary(ks.X(), ks.Ge, ks.RealExpr("0.1"))),
				ks.Check(ks.Binary(ks.X(), ks.Ge, ks.RealExpr("0.2"))),
			),
			value: ks.LitReal("0.15"),
		},
		{
			// NOTE(i4k): this checks internal normalization order.
			name: "(real): weaker lower bound does not replace stronger lower bound",
			schema: ks.With(ks.Real(),
				ks.Check(ks.Binary(ks.X(), ks.Ge, ks.RealExpr("0.2"))),
				ks.Check(ks.Binary(ks.X(), ks.Ge, ks.RealExpr("0.1"))),
			),
			value: ks.LitReal("0.15"),
		},
		{
			name: "(real): stronger upper bound replaces weaker upper bound",
			schema: ks.With(ks.Real(),
				ks.Check(ks.Binary(ks.X(), ks.Le, ks.RealExpr("0.2"))),
				ks.Check(ks.Binary(ks.X(), ks.Le, ks.RealExpr("0.1"))),
			),
			value: ks.LitReal("0.15"),
		},
		{
			name: "(real): weaker upper bound does not replace stronger upper bound",
			schema: ks.With(ks.Real(),
				ks.Check(ks.Binary(ks.X(), ks.Le, ks.RealExpr("0.1"))),
				ks.Check(ks.Binary(ks.X(), ks.Le, ks.RealExpr("0.2"))),
			),
			value: ks.LitReal("0.15"),
		},
		{
			name: "(real): strict lower bound replaces inclusive bound at the same edge",
			schema: ks.With(ks.Real(),
				ks.Check(ks.Binary(ks.X(), ks.Ge, ks.RealExpr("1"))),
				ks.Check(ks.Binary(ks.X(), ks.Gt, ks.RealExpr("1"))),
			),
			value: ks.LitReal("1"),
		},
		{
			name: "(real): inclusive lower bound does not weaken a strict bound at the same edge",
			schema: ks.With(ks.Real(),
				ks.Check(ks.Binary(ks.X(), ks.Gt, ks.RealExpr("1"))),
				ks.Check(ks.Binary(ks.X(), ks.Ge, ks.RealExpr("1"))),
			),
			value: ks.LitReal("1"),
		},
		{
			name: "(real): strict upper bound replaces inclusive bound at the same edge",
			schema: ks.With(ks.Real(),
				ks.Check(ks.Binary(ks.X(), ks.Le, ks.RealExpr("1"))),
				ks.Check(ks.Binary(ks.X(), ks.Lt, ks.RealExpr("1"))),
			),
			value: ks.LitReal("1"),
		},
		{
			name: "(real): inclusive upper bound does not weaken a strict bound at the same edge",
			schema: ks.With(ks.Real(),
				ks.Check(ks.Binary(ks.X(), ks.Lt, ks.RealExpr("1"))),
				ks.Check(ks.Binary(ks.X(), ks.Le, ks.RealExpr("1"))),
			),
			value: ks.LitReal("1"),
		},
		{
			name: "(real): equal inclusive bounds accept the singleton edge",
			schema: ks.With(ks.Real(),
				ks.Check(ks.Binary(ks.X(), ks.Ge, ks.RealExpr("1"))),
				ks.Check(ks.Binary(ks.X(), ks.Le, ks.RealExpr("1"))),
			),
			value: ks.LitReal("1.0"),
			want:  true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := NewArena()
			typ := compile(t, a, tc.schema)
			tree, val := build(t, tc.value)
			if got := a.Valid(typ, tree, val); got != tc.want {
				t.Fatalf("Valid = %v, want %v", got, tc.want)
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
				ks.LitFloat(10),
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
				ks.LitFloat(10),
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
				ks.LitFloat(10),
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
				ks.LitFloat(10),
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
				ks.LitFloat(10),
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
				ks.LitFloat(10),
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
				ks.LitFloat(10),
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
				ks.LitFloat(10),
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
				ks.LitFloat(10),
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
