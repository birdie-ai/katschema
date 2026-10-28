package compiled

import (
	"testing"

	"github.com/birdie-ai/katschema/ks"
)

func TestLiteralASTValidationZeroAllocs(t *testing.T) {
	a := NewArena()
	core := a.Type(compile(t, a, ks.Object(ks.Field("id", ks.String()))))
	b := NewArena()
	patch := b.Type(compile(t, b, ks.Object(ks.Field("priority", ks.Int()))))
	overlay, err := Overlay(core, patch)
	if err != nil {
		t.Fatal(err)
	}

	type testcase struct {
		name  string
		typ   Type
		value ks.Value
		valid bool
	}

	for _, tc := range []testcase{
		{"core", core, ks.Object(ks.Field("id", ks.LitString("one"))), true},
		{"overlay", overlay, ks.Object(ks.Field("id", ks.LitString("one")), ks.Field("priority", ks.LitInt(1))), true},
		{"overlay invalid", overlay, ks.Object(ks.Field("id", ks.LitString("one")), ks.Field("priority", ks.LitString("wrong"))), false},
		{"any", a.Type(a.Any()), ks.Object(ks.Field("values", ks.List(ks.LitInt(1), ks.LitString("one")))), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tree, root, err := ks.Build(tc.value)
			if err != nil {
				t.Fatal(err)
			}
			var valid bool
			allocs := testing.AllocsPerRun(100, func() {
				valid = tc.typ.Valid(tree, root)
			})
			if valid != tc.valid {
				t.Fatalf("Valid = %v, want %v", valid, tc.valid)
			}
			if allocs != 0 {
				t.Fatalf("literal AST validation allocated %v times, want 0", allocs)
			}
		})
	}
}
