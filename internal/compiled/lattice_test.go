package compiled_test

import (
	"testing"

	"github.com/birdie-ai/katschema/internal/compiled"
	"github.com/birdie-ai/katschema/ks"
	"github.com/birdie-ai/katschema/parser/ast"
)

func TestLubz(t *testing.T) {
	t.Parallel()

	type testcase struct {
		name string
		x    ks.Value
		y    ks.Value
		z    ks.Value
	}

	for _, tc := range []testcase{
		{
			name: "LUB((never), T) == T",
			x:    ks.Never(),
			y:    ks.Int(),
			z:    ks.Int(),
		},
		{
			name: "LUB((never), T) == T",
			x:    ks.Never(),
			y:    ks.String(),
			z:    ks.String(),
		},
		{
			name: "LUB((int), (int)) == (int)",
			x:    ks.Int(),
			y:    ks.Int(),
			z:    ks.Int(),
		},
		{
			name: "LUB((int), (string)) == (int)|(string)",
			x:    ks.Int(),
			y:    ks.String(),
			z:    ks.Sum(ks.Int(), ks.String()),
		},
		{
			name: "LUB((int), (int, x > 0)) == (int) because y <= x",
			x:    ks.Int(),
			y:    ks.With(ks.Int(), ks.Check(ks.Binary(ks.X(), ks.Gt, ks.IntExpr(0)))),
			z:    ks.Int(),
		},
		{
			name: "LUB((int, x < 0), (int, x > 0)) == (int, x < 0)|(int, x > 0)",
			x:    ks.With(ks.Int(), ks.Check(ks.Binary(ks.X(), ks.Lt, ks.IntExpr(0)))),
			y:    ks.With(ks.Int(), ks.Check(ks.Binary(ks.X(), ks.Gt, ks.IntExpr(0)))),
			z: ks.Sum(
				ks.With(ks.Int(), ks.Check(ks.Binary(ks.X(), ks.Lt, ks.IntExpr(0)))),
				ks.With(ks.Int(), ks.Check(ks.Binary(ks.X(), ks.Gt, ks.IntExpr(0)))),
			),
		},
		{
			name: "LUB(x, y) = y when x <= y also in objects",
			x: ks.Object(
				ks.Field("a", ks.With(
					ks.Int(),
					ks.Check(
						ks.Binary(ks.X(), ks.Gt, ks.IntExpr(0)),
					),
				),
				),
			),
			y: ks.Object(
				ks.Field("a", ks.Int()),
			),
			z: ks.Object(
				ks.Field("a", ks.Int()),
			),
		},
		{
			name: "optional fields of an object also optimize result",
			x:    ks.Object(ks.Field("a", ks.Int())),
			y: ks.Object(
				ks.Field("a", ks.Int()),
				ks.Field("b", ks.Optional(ks.Int())),
			),
			z: ks.Object(
				ks.Field("a", ks.Int()),
				ks.Field("b", ks.Optional(ks.Int())),
			),
		},
		{
			name: "incompatible objects become a sum",
			x: ks.Object(
				ks.Field("a", ks.Int()),
				ks.Field("b", ks.Optional(ks.Int())),
			),
			y: ks.Object(
				ks.Field("c", ks.Int()),
				ks.Field("d", ks.Optional(ks.Int())),
			),
			z: ks.Sum(
				ks.Object(
					ks.Field("a", ks.Int()),
					ks.Field("b", ks.Optional(ks.Int())),
				),
				ks.Object(
					ks.Field("c", ks.Int()),
					ks.Field("d", ks.Optional(ks.Int())),
				),
			),
		},
		{
			name: "LUB(x, y) = y when x <= y also in lists",
			x: ks.List(ks.With(
				ks.Int(),
				ks.Check(
					ks.Binary(ks.X(), ks.Gt, ks.IntExpr(0)),
				),
			),
			),
			y: ks.List(ks.Int()),
			z: ks.List(ks.Int()),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tree := ast.New()
			xn, err := ks.Emit(tree, tc.x)
			if err != nil {
				t.Fatal(err)
			}
			yn, err := ks.Emit(tree, tc.y)
			if err != nil {
				t.Fatal(err)
			}
			zn, err := ks.Emit(tree, tc.z)
			if err != nil {
				t.Fatal(err)
			}

			arena := compiled.NewArena()
			xc, err := compiled.Compile(arena, tree, xn)
			if err != nil {
				t.Fatal(err)
			}
			yc, err := compiled.Compile(arena, tree, yn)
			if err != nil {
				t.Fatal(err)
			}
			zc, err := compiled.Compile(arena, tree, zn)
			if err != nil {
				t.Fatal(err)
			}
			got := arena.LUB(xc, yc)
			if got != zc {
				t.Fatalf("LUB(%d, %d) = %d (%s), want = %d (%s)",
					xc, yc, got, arena.Type(got).Kind(), zc, arena.Type(zc).Kind(),
				)
			}
		})
	}
}
