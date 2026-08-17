package compiled

import (
	"testing"

	"github.com/birdie-ai/katschema/ks"
	"github.com/birdie-ai/katschema/parser/ast"
)

func TestSubtype(t *testing.T) {
	t.Parallel()

	type testcase struct {
		name string
		a    ks.Value
		b    ks.Value
		want bool
	}

	for _, tc := range []testcase{
		{
			// NOTE(i4k): Subtype(a, b) returns true when all values accepted by a, are also
			// accepted by b. Or, is `a` a subtype of `b`.
			// Subtype((never), (any)) == true because (never) accepts no value (empty set),
			// then b accepts the same empty set.
			// This is an important property later when impossible constraints compile to (never).
			// Otherwise (never) become a strange type that require special handling everywhere.
			name: "debatable but mathematically correct... see comment above",
			a:    ks.Never(),
			b:    ks.Any(),
			want: true,
		},
		{
			name: "any type is bigger than (never)",
			a:    ks.Never(),
			b:    ks.Int(),
			want: true,
		},
		{
			name: "(never) has no subtype",
			a:    ks.Int(),
			b:    ks.Never(),
			want: false,
		},
		{
			name: "(never) has really no subtype",
			a:    ks.Any(),
			b:    ks.Never(),
			want: false,
		},
		{
			name: "any type is subtype of itself",
			a:    ks.Int(),
			b:    ks.Int(),
			want: true,
		},
		{
			name: "(int, x > 0) smaller than (int)",
			a:    ks.With(ks.Int(), ks.Check(ks.Binary(ks.X(), ks.Gt, ks.IntExpr(ks.LitInt(0))))),
			b:    ks.Int(),
			want: true,
		},
		{
			name: "(int) is not smaller than (int, x > 0)",
			a:    ks.Int(),
			b:    ks.With(ks.Int(), ks.Check(ks.Binary(ks.X(), ks.Gt, ks.IntExpr(ks.LitInt(0))))),
			want: false,
		},
		{
			name: "objects with same fields but the less constrained is a subtype",
			a: ks.Object(
				ks.Field("a", ks.With(
					ks.Int(),
					ks.Check(
						ks.Binary(ks.X(), ks.Gt, ks.IntExpr(ks.LitInt(0))),
					),
				),
				),
			),
			b: ks.Object(
				ks.Field("a", ks.Int()),
			),
			want: true,
		},
		{
			name: "object with less fields is a subtype",
			a:    ks.Object(ks.Field("a", ks.Int())),
			b: ks.Object(
				ks.Field("a", ks.Int()),
				ks.Field("b", ks.Optional(ks.Int())),
			),
			want: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			arena := NewArena()
			tree := ast.New()
			a := compileOn(t, arena, tree, tc.a)
			b := compileOn(t, arena, tree, tc.b)
			if got := arena.Subtype(a, b); got != tc.want {
				t.Fatalf("Subtype(a, b) = %v, want %v", got, tc.want)
			}
		})
	}
}
