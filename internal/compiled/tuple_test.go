package compiled

import (
	"errors"
	"math/big"
	"testing"
)

func TestTupleValidation(t *testing.T) {
	t.Parallel()
	type testcase struct {
		schema string
		value  string
		valid  bool
	}
	for _, tc := range []testcase{
		{schema: "#{}", value: "[]", valid: true},
		{schema: "#{}", value: "#{1}", valid: false},
		{schema: "#{(int)}", value: "#{1}", valid: true},
		{schema: "#{(int)}", value: "[1, 2]", valid: false},
		{schema: "[(int)]", value: "#{1, 2}", valid: true},
		{schema: "#{(string), (int)}", value: `["id", 1]`, valid: true},
		{schema: "#{(string), (int)}", value: `#{1, "id"}`, valid: false},
		{schema: "#{(string), (int)}", value: `#{"id"}`, valid: false},
		{schema: "#{(string), (int)}", value: `{"id": 1}`, valid: false},
		{schema: `#{(int, x > 0), #{"end"}}`, value: `#{2, ["end"]}`, valid: true},
		{schema: `#{(int, x > 0), #{"end"}}`, value: `[0, #{"end"}]`, valid: false},
		{schema: "#{(int) | (string), (bool)}", value: `["a", false]`, valid: true},
		{schema: "(#{(int)} | string)", value: "#{3}", valid: true},
		{schema: "([#{(int)}], len(x) >= 1)", value: "[[1], #{2}]", valid: true},
		{schema: "[#{(int)}]", value: "[[1], []]", valid: false},
		{schema: "[#{1}]", value: "[[1], [1]]", valid: false},
		{schema: "(#{(int)}, len(x) >= 1)", value: "#{1}", valid: true},
		{schema: "(#{(int)}, len(x) < 1)", value: "#{1}", valid: false},
		{schema: "(int, x in #{1, 2})", value: "2", valid: true},
		{schema: "(int, x in #{1, 2})", value: "3", valid: false},
		{schema: "(any)", value: `#{{"a": #{null}}, []}`, valid: true},
		{schema: "(any)", value: `#{{"a": (int) | (bool)}}`, valid: false},
	} {
		t.Run(tc.schema+"/"+tc.value, func(t *testing.T) {
			a := NewArena()
			schema := a.Type(compileText(t, a, tc.schema))
			tree, root := parseText(t, tc.value)
			if got := schema.Valid(tree, root); got != tc.valid {
				t.Errorf("Valid = %t, want %t", got, tc.valid)
			}
			b := NewArena()
			value := b.Type(compileText(t, b, tc.value))
			if got := schema.ValidType(value); got != tc.valid {
				t.Errorf("ValidType = %t, want %t", got, tc.valid)
			}
		})
	}
}

func TestTupleTypeCheck(t *testing.T) {
	t.Parallel()
	a := NewArena()
	schema := compileText(t, a, `(#{(real), (string)}, len(x) <= 2, name="measurement")`)
	value := compileText(t, a, `[2, "m"]`)
	got, err := a.TypeCheck(schema, value)
	if err != nil {
		t.Fatal(err)
	}
	if want := compileText(t, a, `[2.0, "m"]`); got != want {
		t.Fatal("TypeCheck did not interpret the first tuple position as real")
	}
	if _, err := a.TypeCheck(schema, compileText(t, a, `["m", 2]`)); err == nil {
		t.Fatal("TypeCheck accepted swapped tuple positions")
	}
	if name, ok := a.Type(schema).Metadata().Get("name"); !ok || name != "measurement" {
		t.Fatalf("tuple metadata = %v, found = %v", name, ok)
	}
}

func TestTupleMetadataValue(t *testing.T) {
	t.Parallel()
	a := NewArena()
	schema := a.Type(compileText(t, a, `(int, sample=#{9223372036854775808, 1})`))
	sample, ok := schema.Metadata().Get("sample")
	elems, isTuple := sample.([]any)
	if !ok || !isTuple || len(elems) != 2 {
		t.Fatalf("sample = %#v, want two tuple elements", sample)
	}
	large, ok := elems[0].(*big.Int)
	if !ok || large.String() != "9223372036854775808" || elems[1] != int64(1) {
		t.Fatalf("tuple metadata values = %#v", elems)
	}
}

func TestTupleSubtype(t *testing.T) {
	t.Parallel()
	a, b := NewArena(), NewArena()
	single := a.Type(compileText(t, a, "#{(int)}"))
	list := b.Type(compileText(t, b, "[(int)]"))
	if single.Kind() != Tuple || list.Kind() != List {
		t.Fatalf("kinds = %s, %s, want Tuple, List", single.Kind(), list.Kind())
	}
	if !single.SubtypeOf(list) || list.SubtypeOf(single) {
		t.Fatal("a one-position integer tuple must be a strict subtype of an integer list")
	}
	if got := compileText(t, a, "(#{(int)}, len(x) >= 1)"); got != single.ID() {
		t.Fatal("satisfied length bound changed the type of the tuple")
	}
	for _, src := range []string{"(#{(int)}, len(x) < 1)", "#{(never)}", "#{(string), #{(never)}}"} {
		if got := compileText(t, a, src); got != a.Never() {
			t.Errorf("%s = %s, want Never", src, a.Type(got).Kind())
		}
	}
}

func TestTupleCompileErrors(t *testing.T) {
	t.Parallel()
	for _, src := range []string{"#{(int, optional)}", "(#{(int, optional)})"} {
		tree, root := parseText(t, src)
		_, err := Compile(NewArena(), tree, root)
		if !errors.Is(err, ErrOptionalUnexpected) {
			t.Errorf("Compile(%q) = %v, want ErrOptionalUnexpected", src, err)
		}
	}
}

func TestTupleRejectsSchemaAsData(t *testing.T) {
	t.Parallel()
	a := NewArena()
	for _, src := range []string{
		"#{(int, x == 1)}",
		`#{{"x": (int, x == 1) | (int, x == 1)}}`,
	} {
		tree, root := parseText(t, src)
		if a.Valid(a.Any(), tree, root) {
			t.Errorf("Any accepted schema syntax inside tuple data %s", src)
		}
	}
}
