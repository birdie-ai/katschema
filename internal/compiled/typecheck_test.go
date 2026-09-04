package compiled

import (
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
				checkDecimal(t, a, got, "10", RealAtom)
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
				checkDecimal(t, a, got, "31.5", RealAtom)
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

func checkDecimal(t *testing.T, a *Arena, got TypeID, want string, wantKind Kind) {
	atom, ok := a.Literal(got)
	if !ok || a.Node(atom).Kind() != wantKind {
		t.Fatalf("typed literal atom = %d, %t, want a real literal", atom, ok)
	}
	gotVal, ok := a.DecimalValue(atom)
	if !ok {
		t.Fatalf("not a decimal: %d", got)
	}
	if want := decimal.New(want); !gotVal.Equal(want) {
		t.Fatalf("got [%s] != want [%s]", gotVal, want)
	}
}
