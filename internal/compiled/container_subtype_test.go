package compiled

import (
	"testing"

	"github.com/birdie-ai/katschema/ks"
)

func TestContainerSubtype(t *testing.T) {
	t.Parallel()

	length := func(value ks.Value, op ks.Op, n int64) ks.Value {
		return ks.Where(value, ks.Binary(ks.Funcall("len", ks.X()), op, ks.IntExpr(n)))
	}
	ints := ks.List(ks.Int())
	nonempty := length(ints, ks.Gt, 0)
	one := ks.List(ks.LitInt(1))
	object := ks.Object(ks.Field("id", ks.Int()))
	optional := ks.Object(ks.Field("id", ks.Optional(ks.Int())))
	optionalExtra := ks.Object(ks.Field("id", ks.Optional(ks.Int())), ks.Field("extra", ks.Optional(ks.Int())))
	withExtra := ks.Object(ks.Field("id", ks.Int()), ks.Field("extra", ks.Optional(ks.Int())))

	type testcase struct {
		name string
		a, b ks.Value
		want bool
	}

	for _, tc := range []testcase{
		{"nonempty tuple", one, nonempty, true},
		{"empty tuple", ks.List(), nonempty, false},
		{"wrong element", ks.List(ks.LitString("one")), nonempty, false},
		{"tuple at limit", one, length(ints, ks.Le, 1), true},
		{"tuple over limit", ks.List(ks.LitInt(1), ks.LitInt(2)), length(ints, ks.Le, 1), false},
		{"broad list", ints, nonempty, false},
		{"unbounded list", nonempty, length(ints, ks.Le, 1), false},
		{"narrow list elements", length(ks.List(ks.Where(ks.Int(), ks.Binary(ks.X(), ks.Gt, ks.IntExpr(0)))), ks.Ge, 2), nonempty, true},
		{"source list refinement", length(one, ks.Ge, 1), nonempty, true},
		{"metadata object", ks.With(object, ks.Flag("source")), ks.With(length(object, ks.Gt, 0), ks.Flag("target")), true},
		{"required object", object, length(object, ks.Gt, 0), true},
		{"optional target", object, length(optional, ks.Gt, 0), true},
		{"optional source may be empty", optional, length(optional, ks.Gt, 0), false},
		{"optional source within maximum", optional, length(optional, ks.Le, 1), true},
		{"required field missing", optional, length(object, ks.Le, 1), false},
		{"source object refinement", length(optional, ks.Ge, 1), length(optional, ks.Gt, 0), true},
		{"source object fields imply minimum", length(object, ks.Ge, 0), length(object, ks.Gt, 0), true},
		{"source object with narrower fields", length(optional, ks.Ge, 1), length(optionalExtra, ks.Gt, 0), true},
		{"object field type", ks.Object(ks.Field("id", ks.String())), length(object, ks.Gt, 0), false},
		{"object over limit", withExtra, length(withExtra, ks.Le, 1), false},
		{"never remains subtype", ks.Never(), nonempty, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, separate := range []bool{false, true} {
				a := NewArena()
				b := a
				if separate {
					b = NewArena()
				}
				left := a.Type(compile(t, a, tc.a))
				right := b.Type(compile(t, b, tc.b))
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
	bottom := a.Type(compile(t, a, ks.With(ks.Object(ks.Field("id", ks.LitInt(1))), ks.Flag("source"))))
	top := b.Type(compile(t, b, ks.Object(ks.Field("name", ks.LitString("one")))))
	value, err := Overlay(bottom, top)
	if err != nil {
		t.Fatal(err)
	}
	object := ks.Object(ks.Field("id", ks.Int()), ks.Field("name", ks.String()))
	for _, limit := range []int64{1, 2} {
		schema := c.Type(compile(t, c, ks.Where(object, ks.Binary(ks.Funcall("len", ks.X()), ks.Le, ks.IntExpr(limit)))))
		if got := value.SubtypeOf(schema); got != (limit == 2) {
			t.Fatalf("SubtypeOf len <= %d = %v", limit, got)
		}
	}
}

func TestConstraintAcceptsContainers(t *testing.T) {
	t.Parallel()

	a, b := NewArena(), NewArena()
	schema := a.Type(compile(t, a, ks.Where(ks.List(ks.Int()), ks.Binary(ks.Binary(ks.Funcall("len", ks.X()), ks.Ge, ks.IntExpr(1)), ks.And, ks.Binary(ks.Funcall("len", ks.X()), ks.Le, ks.IntExpr(1))))))

	type testcase struct {
		value ks.Value
		want  bool
	}

	for _, tc := range []testcase{
		{ks.List(ks.LitInt(1)), true},
		{ks.List(), false},
		{ks.Object(ks.Field("id", ks.LitInt(1))), true},
		{ks.Object(), false},
	} {
		value := b.Type(compile(t, b, tc.value))
		if got := schema.constraint().accepts(value); got != tc.want {
			t.Fatalf("constraint accepts %s = %v, want %v", value.Kind(), got, tc.want)
		}
	}
	enum := a.Type(compile(t, a, ks.LitInt(1)))
	value := b.Type(compile(t, b, ks.List(ks.LitInt(1))))
	if enum.constraint().accepts(value) {
		t.Fatal("scalar enum accepted tuple")
	}
}
