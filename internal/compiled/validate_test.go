package compiled

import (
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

	/*
		TODO(i4k): not yet! In this PR, the (int) type is not the mathematical Z type.
		var nextGoogleBigInt big.Int
		nextGoogleBigInt.Add(&googleBigInt, big.NewInt(1))

		// (int, 0 <= x <= 10^100)
		bigIntRange := ks.Where(ks.Int(), ks.Binary(
			ks.Binary(ks.IntExpr(0), ks.Le, ks.X()),
			ks.Le,
			ks.ValueExpr(ks.LitBigInt(&googleBigInt)),
		))
	*/

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
		/*
			     * TODO(i4k): not yet! In this PR, the (int) type is not the mathematical Z type.
				 * {
				 * 	name:   "valid upper bound in bigInt range",
				 * 	schema: bigIntRange,
				 * 	value:  ks.LitBigInt(&googleBigInt),
				 * 	want:   true,
				 * },
		*/
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
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := NewArena()
			typ := compile(t, a, tc.schema)
			values, root := build(t, tc.value)
			if got := a.Valid(typ, values, root); got != tc.want {
				t.Fatalf("Valid = %v, want %v", got, tc.want)
			}
		})
	}
}
