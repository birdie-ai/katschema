package katschema_test

import (
	"strings"
	"testing"

	"github.com/birdie-ai/katschema"
)

func TestTupleAccess(t *testing.T) {
	t.Parallel()

	values := []string{
		"2",
		"1",
		"2",
		"null",
		"false",
		`"birdie"`,
		"1.25",
		"[]",
		"[true]",
		`{"name": "birdie"}`,
	}
	tuple := compileText(t, katschema.NewCompiler(), "["+strings.Join(values, ",")+"]")
	if tuple.Kind() != katschema.Tuple || tuple.Len() != len(values) {
		t.Fatalf("kind=%v len=%d, want tuple of length %d", tuple.Kind(), tuple.Len(), len(values))
	}
	for i, value := range values {
		got := tuple.At(i)
		want := compileText(t, katschema.NewCompiler(), value)
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
			{name: "zero type", typ: katschema.Type{}, len: 0},
			{name: "list schema", typ: compileText(t, cc, "[(int)]"), len: 0},
			{name: "scalar", typ: compileText(t, cc, "1"), len: 0},
			{name: "object", typ: compileText(t, cc, `{"id": 1}`), len: 1},
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
		tuple := compileText(t, cc, "[2, 1]")
		if tuple.Kind() != katschema.Tuple || tuple.Len() != 2 {
			t.Fatalf("kind=%v len=%d, want tuple of length 2", tuple.Kind(), tuple.Len())
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

	bottom := compileText(t, katschema.NewCompiler(), `{"kept": ["bottom"], "replaced": ["old"]}`)
	top := compileText(t, katschema.NewCompiler(), `{"replaced": ["top"]}`)
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
