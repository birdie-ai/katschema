package compiled

import (
	"bytes"
	"testing"

	"github.com/birdie-ai/katschema/ks"
	"github.com/birdie-ai/katschema/math/decimal"
)

func TestTypeCheck(t *testing.T) {
	t.Parallel()

	type testcase struct {
		name      string
		schema    ks.Value
		value     ks.Value
		check     func(*testing.T, *Arena, TypeID)
		wantError bool
	}

	for _, tc := range []testcase{
		{
			name:   "literal keeps int context",
			schema: ks.Int(),
			value:  ks.LitInt(10),
			check: func(t *testing.T, a *Arena, got TypeID) {
				if got != compile(t, a, ks.LitInt(10)) {
					t.Fatalf("typed literal = %d, want the canonical int literal", got)
				}
			},
		},
		{
			name:   "integer is interpreted as real",
			schema: ks.Real(),
			value:  ks.LitInt(10),
			check: func(t *testing.T, a *Arena, got TypeID) {
				if a.Type(got).Base() != a.Real() {
					t.Fatalf("typed literal base = %d, want real %d", a.Type(got).Base(), a.Real())
				}
				atom, ok := a.Literal(got)
				if !ok || a.Node(atom).Kind() != RealAtom {
					t.Fatalf("typed literal atom = %d, %t, want a real literal", atom, ok)
				}

				gotNumber, ok := a.DecimalValue(atom)
				if !ok {
					t.Fatalf("typed literal atom = %d is not a decimal value", atom)
				}
				wantNumber, err := decimal.Parse([]byte("10.0"), nil)
				if err != nil {
					t.Fatal(err)
				}
				if gotNumber.Neg != wantNumber.Neg ||
					gotNumber.Exp != wantNumber.Exp ||
					!bytes.Equal(gotNumber.Digits, wantNumber.Digits) {
					t.Fatalf("typed literal atom = %#v, want %#v", gotNumber, wantNumber)
				}
			},
		},
		{
			name:   "decimal is interpreted as float32",
			schema: ks.Where(ks.Float32(), ks.Binary(ks.X(), ks.Lt, ks.DecimalExpr("100"))),
			value:  ks.LitDecimal("31.5"),
			check: func(t *testing.T, a *Arena, got TypeID) {
				if a.Type(got).Base() != a.Float32() {
					t.Fatalf("typed literal base = %d, want float32 %d", a.Type(got).Base(), a.Float32())
				}
				if _, ok := a.Literal(got); !ok {
					t.Fatalf("typed value %d is not a literal", got)
				}
			},
		},
		{
			name: "object fields are interpreted recursively",
			schema: ks.Object(
				ks.Field("temp", ks.Float32()),
				ks.Field("name", ks.String()),
			),
			value: ks.Object(
				ks.Field("temp", ks.LitDecimal("31.5")),
				ks.Field("name", ks.LitString("weather")),
			),
			check: func(t *testing.T, a *Arena, got TypeID) {
				fields := a.Type(got).Fields()
				temp := fields.MustGet("temp")
				name := fields.MustGet("name")
				if tempBase := a.Type(temp.Type()).Base(); tempBase != a.Float32() {
					t.Fatalf("temp base = %d, want float32 %d", tempBase, a.Float32())
				}
				if nameType := a.Type(name.Type()).Base(); nameType != a.String() {
					t.Fatalf("name is not a string: %d", nameType)
				}
			},
		},
		{
			name:      "value outside schema is rejected",
			schema:    ks.Int(),
			value:     ks.LitDecimal("10.5"),
			wantError: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := NewArena()
			schema := compile(t, a, tc.schema)
			value := compile(t, a, tc.value)
			got, err := a.TypeCheck(schema, value)
			if tc.wantError {
				if err == nil {
					t.Fatalf("TypeCheck(%d, %d) unexpectedly succeeded with %d", schema, value, got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !a.Subtype(got, schema) {
				t.Fatalf("typed value %d is not a subtype of schema %d", got, schema)
			}
			tc.check(t, a, got)
		})
	}
}
