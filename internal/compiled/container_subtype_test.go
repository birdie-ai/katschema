package compiled

import "testing"

func TestContainerSubtype(t *testing.T) {
	t.Parallel()

	type testcase struct {
		name string
		a, b string
		want bool
	}

	for _, tc := range []testcase{
		{name: "nonempty tuple", a: "[1]", b: "([(int)], len(x) > 0)", want: true},
		{name: "empty tuple", a: "[]", b: "([(int)], len(x) > 0)", want: false},
		{name: "wrong element", a: `["one"]`, b: "([(int)], len(x) > 0)", want: false},
		{name: "tuple at limit", a: "[1]", b: "([(int)], len(x) <= 1)", want: true},
		{name: "tuple over limit", a: "[1, 2]", b: "([(int)], len(x) <= 1)", want: false},
		{name: "broad list", a: "[(int)]", b: "([(int)], len(x) > 0)", want: false},
		{name: "unbounded list", a: "([(int)], len(x) > 0)", b: "([(int)], len(x) <= 1)", want: false},
		{name: "narrow list elements", a: "([(int, x > 0)], len(x) >= 2)", b: "([(int)], len(x) > 0)", want: true},
		{name: "source list refinement", a: "([1], len(x) >= 1)", b: "([(int)], len(x) > 0)", want: true},
		{
			name: "metadata object",
			a:    `({"id": (int)}, source)`,
			b:    `({"id": (int)}, len(x) > 0, target)`,
			want: true,
		},
		{
			name: "required object",
			a:    `{"id": (int)}`,
			b:    `({"id": (int)}, len(x) > 0)`,
			want: true,
		},
		{
			name: "optional target",
			a:    `{"id": (int)}`,
			b:    `({"id": (int, optional)}, len(x) > 0)`,
			want: true,
		},
		{
			name: "optional source may be empty",
			a:    `{"id": (int, optional)}`,
			b:    `({"id": (int, optional)}, len(x) > 0)`,
			want: false,
		},
		{
			name: "optional source within maximum",
			a:    `{"id": (int, optional)}`,
			b:    `({"id": (int, optional)}, len(x) <= 1)`,
			want: true,
		},
		{
			name: "required field missing",
			a:    `{"id": (int, optional)}`,
			b:    `({"id": (int)}, len(x) <= 1)`,
			want: false,
		},
		{
			name: "source object refinement",
			a:    `({"id": (int, optional)}, len(x) >= 1)`,
			b:    `({"id": (int, optional)}, len(x) > 0)`,
			want: true,
		},
		{
			name: "source object fields imply minimum",
			a:    `({"id": (int)}, len(x) >= 0)`,
			b:    `({"id": (int)}, len(x) > 0)`,
			want: true,
		},
		{
			name: "source object with narrower fields",
			a:    `({"id": (int, optional)}, len(x) >= 1)`,
			b:    `({"id": (int, optional), "extra": (int, optional)}, len(x) > 0)`,
			want: true,
		},
		{
			name: "object field type",
			a:    `{"id": (string)}`,
			b:    `({"id": (int)}, len(x) > 0)`,
			want: false,
		},
		{
			name: "object over limit",
			a:    `{"id": (int), "extra": (int, optional)}`,
			b:    `({"id": (int), "extra": (int, optional)}, len(x) <= 1)`,
			want: false,
		},
		{name: "never remains subtype", a: "(never)", b: "([(int)], len(x) > 0)", want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, separate := range []bool{false, true} {
				a := NewArena()
				b := a
				if separate {
					b = NewArena()
				}
				left := a.Type(compileText(t, a, tc.a))
				right := b.Type(compileText(t, b, tc.b))
				if got := left.SubtypeOf(right); got != tc.want {
					t.Fatalf("SubtypeOf = %v, want %v (separate arenas: %v)", got, tc.want, separate)
				}
			}
		})
	}
}

func TestContainerSubtypeOverlay(t *testing.T) {
	t.Parallel()

	a, b, c := NewArena(), NewArena(), NewArena()
	bottom := a.Type(compileText(t, a, `({"id": 1}, source)`))
	top := b.Type(compileText(t, b, `{"name": "one"}`))
	value, err := Overlay(bottom, top)
	if err != nil {
		t.Fatal(err)
	}
	type testcase struct {
		schema string
		want   bool
	}
	for _, tc := range []testcase{
		{schema: `({"id": (int), "name": (string)}, len(x) <= 1)`, want: false},
		{schema: `({"id": (int), "name": (string)}, len(x) <= 2)`, want: true},
	} {
		schema := c.Type(compileText(t, c, tc.schema))
		if got := value.SubtypeOf(schema); got != tc.want {
			t.Fatalf("SubtypeOf %s = %v, want %v", tc.schema, got, tc.want)
		}
	}
}

func TestConstraintAcceptsContainers(t *testing.T) {
	t.Parallel()

	a, b := NewArena(), NewArena()
	schema := a.Type(compileText(t, a, "([(int)], len(x) >= 1 && len(x) <= 1)"))

	type testcase struct {
		value string
		want  bool
	}

	for _, tc := range []testcase{
		{value: "[1]", want: true},
		{value: "[]", want: false},
		{value: `{"id": 1}`, want: true},
		{value: "{}", want: false},
	} {
		value := b.Type(compileText(t, b, tc.value))
		if got := schema.constraint().accepts(value); got != tc.want {
			t.Fatalf("constraint accepts %s = %v, want %v", value.Kind(), got, tc.want)
		}
	}
	enum := a.Type(compileText(t, a, "1"))
	value := b.Type(compileText(t, b, "[1]"))
	if enum.constraint().accepts(value) {
		t.Fatal("scalar enum accepted tuple")
	}
}
