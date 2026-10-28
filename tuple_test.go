package katschema_test

import (
	"testing"

	"github.com/birdie-ai/katschema"
	"github.com/birdie-ai/katschema/ks"
)

func TestTupleAccess(t *testing.T) {
	t.Parallel()

	values := []ks.Value{
		ks.LitInt(2),
		ks.LitInt(1),
		ks.LitInt(2),
		ks.LitNull(),
		ks.LitBool(false),
		ks.LitString("birdie"),
		ks.LitDecimal("1.25"),
		ks.List(),
		ks.List(ks.LitBool(true)),
		ks.Object(ks.Field("name", ks.LitString("birdie"))),
	}
	tuple := compile(t, katschema.NewCompiler(), ks.List(values...))
	if tuple.Kind() != katschema.Tuple || tuple.Len() != len(values) {
		t.Fatalf("kind=%v len=%d, want tuple of length %d", tuple.Kind(), tuple.Len(), len(values))
	}
	for i, value := range values {
		got := tuple.At(i)
		want := compile(t, katschema.NewCompiler(), value)
		if !got.Equivalent(want) {
			t.Fatalf("At(%d) differs from the source element", i)
		}
	}

	nested := tuple.At(8)
	if nested.Len() != 1 {
		t.Fatal("missing nested array element")
	}
	item := nested.At(0)
	if value, ok := item.BoolValue(); !ok || !value {
		t.Fatalf("nested element = %v, %v; want true, true", value, ok)
	}
	object := tuple.At(9)
	name, ok := object.Field("name")
	if !ok {
		t.Fatal("missing object field")
	}
	if value, ok := name.Type.StringValue(); !ok || value != "birdie" {
		t.Fatalf("object field = %q, %v; want birdie, true", value, ok)
	}
}

func TestTupleAccessPanics(t *testing.T) {
	t.Parallel()

	t.Run("non-tuple", func(t *testing.T) {
		type testcase struct {
			name string
			typ  katschema.Type
			len  int
		}
		cc := katschema.NewCompiler()
		for _, tc := range []testcase{
			{"zero type", katschema.Type{}, 0},
			{"list schema", compile(t, cc, ks.List(ks.Int())), 0},
			{"scalar", compile(t, cc, ks.LitInt(1)), 0},
			{"object", compile(t, cc, ks.Object(ks.Field("id", ks.LitInt(1)))), 1},
		} {
			t.Run(tc.name, func(t *testing.T) {
				if got := tc.typ.Len(); got != tc.len {
					t.Errorf("Len = %d, want %d", got, tc.len)
				}
				defer func() {
					if recover() == nil {
						t.Fatal("At on a non-tuple did not panic")
					}
				}()
				tc.typ.At(0)
			})
		}
	})

	t.Run("indexing", func(t *testing.T) {
		cc := katschema.NewCompiler()
		values := []ks.Value{
			ks.LitInt(2),
			ks.LitInt(1),
		}
		tuple := compile(t, cc, ks.List(values...))
		if tuple.Kind() != katschema.Tuple || tuple.Len() != len(values) {
			t.Fatalf("kind=%v len=%d, want tuple of length %d", tuple.Kind(), tuple.Len(), len(values))
		}
		for _, index := range []int{
			-1, 3, 10, -10,
		} {
			defer func() {
				if recover() == nil {
					t.Fatal("At on a non-tuple did not panic")
				}
			}()
			tuple.At(index)
		}
	})
}

func TestTupleAccessThroughOverlay(t *testing.T) {
	t.Parallel()

	bottom := compile(t, katschema.NewCompiler(), ks.Object(
		ks.Field("kept", ks.List(ks.LitString("bottom"))),
		ks.Field("replaced", ks.List(ks.LitString("old"))),
	))
	top := compile(t, katschema.NewCompiler(), ks.Object(
		ks.Field("replaced", ks.List(ks.LitString("top"))),
	))
	merged, err := katschema.Overlay(bottom, top)
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"kept": "bottom", "replaced": "top"} {
		field, ok := merged.Field(name)
		if !ok || field.Type.Kind() != katschema.Tuple || field.Type.Len() != 1 {
			t.Fatalf("missing tuple field %q", name)
		}
		item := field.Type.At(0)
		if got, ok := item.StringValue(); !ok || got != want {
			t.Errorf("field %q element = %q, %v; want %q, true", name, got, ok, want)
		}
	}
}
