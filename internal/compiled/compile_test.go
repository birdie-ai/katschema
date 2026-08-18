package compiled

import (
	"testing"

	"github.com/birdie-ai/katschema/ks"
	"github.com/birdie-ai/katschema/parser/ast"
	"github.com/birdie-ai/katschema/parser/token"
)

func TestCompileIntern(t *testing.T) {
	t.Parallel()

	// NOTE(i4k): here we need to use the AST nodes because we need to ensure syntactic
	// differences that are semantically equal lead to same interning. The [ks] DSL is
	// synthetic, then it does not preserve source differences.

	type testcase struct {
		name string
		x    func(*ast.Tree) ast.NodeID
		y    func(*ast.Tree) ast.NodeID
	}

	var z token.Span

	for _, tc := range []testcase{
		{
			name: "same builtin type",
			x: func(a *ast.Tree) ast.NodeID {
				return a.AddSchema(a.AddName("int", z), nil, z)
			},
			y: func(a *ast.Tree) ast.NodeID {
				return a.AddSchema(a.AddName("int", z), nil, z)
			},
		},
		{
			name: "same literal int different syntax",
			x: func(a *ast.Tree) ast.NodeID {
				return a.AddInt("1337", z)
			},
			y: func(a *ast.Tree) ast.NodeID {
				return a.AddInt("+1337", z)
			},
		},
		{
			name: "same literal float different syntax",
			x: func(a *ast.Tree) ast.NodeID {
				return a.AddFloat("1.01", z)
			},
			y: func(a *ast.Tree) ast.NodeID {
				return a.AddFloat("1.01e0", z)
			},
		},
		{
			name: "object order is ignored",
			x: func(a *ast.Tree) ast.NodeID {
				return a.AddObject(
					[]ast.Field{
						a.NewField("id", a.AddInt("1", z), z, z),
						a.NewField("id2", a.AddInt("2", z), z, z),
					},
					z,
				)
			},
			y: func(a *ast.Tree) ast.NodeID {
				return a.AddObject(
					[]ast.Field{
						a.NewField("id2", a.AddInt("2", z), z, z),
						a.NewField("id", a.AddInt("1", z), z, z),
					},
					z,
				)
			},
		},
		{
			name: "metadata is ignored",
			x: func(a *ast.Tree) ast.NodeID {
				return a.AddSchema(
					a.AddName("string", z),
					[]ast.NodeID{a.AddAttr("something", a.AddInt("1337", z), true, z, z)},
					z,
				)
			},
			y: func(a *ast.Tree) ast.NodeID {
				return a.AddSchema(a.AddName("string", z), nil, z)
			},
		},
		{
			name: "A | A => A",
			x: func(a *ast.Tree) ast.NodeID {
				return a.AddSum(
					a.AddSchema(a.AddName("string", z), nil, z),
					a.AddSchema(a.AddName("string", z), nil, z),
					z,
				)
			},
			y: func(a *ast.Tree) ast.NodeID {
				return a.AddSchema(a.AddName("string", z), nil, z)
			},
		},
		{
			name: "A | B | C | A => A | B | C", // redundancy
			x: func(a *ast.Tree) ast.NodeID {
				return a.AddSum(
					a.AddSchema(a.AddName("bool", z), nil, z),
					a.AddSum(
						a.AddSchema(a.AddName("int", z), nil, z),
						a.AddSum(
							a.AddSchema(a.AddName("float", z), nil, z),
							a.AddSchema(a.AddName("bool", z), nil, z),
							z,
						),
						z,
					),
					z,
				)
			},
			y: func(a *ast.Tree) ast.NodeID {
				return a.AddSum(
					a.AddSchema(a.AddName("bool", z), nil, z),
					a.AddSum(
						a.AddSchema(a.AddName("int", z), nil, z),
						a.AddSchema(a.AddName("float", z), nil, z),
						z,
					),
					z,
				)
			},
		},
		{
			name: "A | (never) => A",
			x: func(a *ast.Tree) ast.NodeID {
				return a.AddSum(
					a.AddSchema(a.AddName("string", z), nil, z),
					a.AddSchema(a.AddName("never", z), nil, z),
					z,
				)
			},
			y: func(a *ast.Tree) ast.NodeID {
				return a.AddSchema(a.AddName("string", z), nil, z)
			},
		},
		{
			name: "A | (any) => (any)",
			x: func(a *ast.Tree) ast.NodeID {
				return a.AddSum(
					a.AddSchema(a.AddName("string", z), nil, z),
					a.AddSchema(a.AddName("any", z), nil, z),
					z,
				)
			},
			y: func(a *ast.Tree) ast.NodeID {
				return a.AddSchema(a.AddName("any", z), nil, z)
			},
		},
		{
			name: "A | B => B iff A <= B",
			x: func(a *ast.Tree) ast.NodeID {
				return a.AddSum(
					a.AddInt("1337", z),
					a.AddSchema(a.AddName("int", z), nil, z),
					z,
				)
			},
			y: func(a *ast.Tree) ast.NodeID {
				return a.AddSchema(a.AddName("int", z), nil, z)
			},
		},
		{
			name: "A | B => A iff B <= A",
			x: func(a *ast.Tree) ast.NodeID {
				return a.AddSum(
					a.AddSchema(a.AddName("int", z), nil, z),
					a.AddInt("1337", z),
					z,
				)
			},
			y: func(a *ast.Tree) ast.NodeID {
				return a.AddSchema(a.AddName("int", z), nil, z)
			},
		},
		{
			name: "true | false => (bool)",
			x: func(a *ast.Tree) ast.NodeID {
				return a.AddSum(
					a.AddBool(true, z),
					a.AddBool(false, z),
					z,
				)
			},
			y: func(a *ast.Tree) ast.NodeID {
				return a.AddSchema(a.AddName("bool", z), nil, z)
			},
		},
		{
			name: "true | false | true => (bool)",
			x: func(a *ast.Tree) ast.NodeID {
				return a.AddSum(
					a.AddBool(true, z),
					a.AddSum(
						a.AddBool(false, z),
						a.AddBool(true, z),
						z,
					),
					z,
				)
			},
			y: func(a *ast.Tree) ast.NodeID {
				return a.AddSchema(a.AddName("bool", z), nil, z)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tree := ast.New()
			x := tc.x(tree)
			y := tc.y(tree)

			arena := NewArena()
			xid, err := Compile(arena, tree, x)
			if err != nil {
				t.Fatal(err)
			}
			yid, err := Compile(arena, tree, y)
			if err != nil {
				t.Fatal(err)
			}

			if xid != yid {
				t.Fatalf("IDs differ: %d != %d", xid, yid)
			}
		})
	}
}

func TestConstraintNormalization(t *testing.T) {
	t.Parallel()

	type testcase struct {
		name  string
		cases []ks.Value // cases[0] is considered the wanted case
	}

	a := NewArena()
	x := ks.X()
	zero := ks.IntExpr(0)
	hundred := ks.IntExpr(100)

	for _, tc := range []testcase{
		{
			name: "semantically equal int ranges",
			cases: []ks.Value{
				// (int, x >= 0, x <= 100)
				ks.With(ks.Int(),
					ks.Check(ks.Binary(x, ks.Ge, zero)),
					ks.Check(ks.Binary(x, ks.Le, hundred)),
				),
				// (int, x >= 0 && x <= 100)
				ks.With(ks.Int(), ks.Check(
					ks.Binary(
						ks.Binary(x, ks.Ge, zero),
						ks.And,
						ks.Binary(x, ks.Le, hundred),
					)),
				),
				// (int, 0 <= x <= 100)
				ks.With(ks.Int(), ks.Check(
					ks.Binary(
						ks.Binary(zero, ks.Le, x),
						ks.Le,
						hundred,
					)),
				),
				// NOTE(i4k): we normalize into same bounds.
				// (int, -1 < x <= 100)
				ks.With(ks.Int(), ks.Check(
					ks.Binary(
						ks.Binary(ks.IntExpr(-1), ks.Lt, x),
						ks.Le,
						hundred,
					)),
				),
				// (int, -1 < x < 101)
				ks.With(ks.Int(), ks.Check(
					ks.Binary(
						ks.Binary(ks.IntExpr(-1), ks.Lt, x),
						ks.Lt,
						ks.IntExpr(101),
					)),
				),
			},
		},
		{

			name: "semantically equal float ranges",
			cases: []ks.Value{
				// (float, x >= 0, x <= 100)
				ks.With(ks.Float(),
					ks.Check(ks.Binary(x, ks.Ge, ks.FloatExpr(0))),
					ks.Check(ks.Binary(x, ks.Le, ks.FloatExpr(100))),
				),
				// (float, x >= 0 && x <= 100)
				ks.With(ks.Float(), ks.Check(
					ks.Binary(
						ks.Binary(x, ks.Ge, ks.FloatExpr(0)),
						ks.And,
						ks.Binary(x, ks.Le, ks.FloatExpr(100)),
					)),
				),
				// (float, 0 <= x <= 100)
				ks.With(ks.Float(), ks.Check(
					ks.Binary(
						ks.Binary(ks.FloatExpr(0), ks.Le, x),
						ks.Le,
						ks.FloatExpr(100),
					)),
				),
			},
		},
		{
			name: "semantically equal enums",
			cases: []ks.Value{
				// (string, x IN ["a", "b", "c"])
				ks.With(ks.String(), ks.Check(
					ks.Binary(
						ks.X(),
						ks.In,
						ks.ListExpr(ks.LitString("a"), ks.LitString("b"), ks.LitString("c")),
					),
				)),
				// (string, x IN ["b", "a", "c"])
				ks.With(ks.String(), ks.Check(
					ks.Binary(
						ks.X(),
						ks.In,
						ks.ListExpr(ks.LitString("b"), ks.LitString("a"), ks.LitString("c")),
					),
				)),
				// (string, x IN ["c", "b", "a"])
				ks.With(ks.String(), ks.Check(
					ks.Binary(
						ks.X(),
						ks.In,
						ks.ListExpr(ks.LitString("c"), ks.LitString("b"), ks.LitString("a")),
					),
				)),

				// NOTE(i4k): EQ and IN canonicalize the same.

				// (string, x == "a", x == "b", x == "c")
				ks.With(ks.String(), ks.Check(
					ks.Binary(
						ks.X(),
						ks.In,
						ks.ListExpr(ks.LitString("c"), ks.LitString("b"), ks.LitString("a")),
					),
				)),
				// NOTE(i4k): this is handled as a special case in the implementation for now.
				// I'm still not sure how to generalize that.
				// (string, x == "a" || x == "b" || x == "c")
				ks.Where(
					ks.String(),
					ks.Binary(
						ks.Binary(
							ks.Binary(
								ks.X(),
								ks.Eq,
								ks.StrExpr("a"),
							),
							ks.Or,
							ks.Binary(
								ks.X(),
								ks.Eq,
								ks.StrExpr("b"),
							),
						),
						ks.Or,
						ks.Binary(
							ks.X(),
							ks.Eq,
							ks.StrExpr("c"),
						),
					),
				),
				// (string, x IN ["c", "b"], x == "a")
				ks.With(ks.String(), ks.Check(
					ks.Binary(
						ks.X(),
						ks.In,
						ks.ListExpr(ks.LitString("c"), ks.LitString("b"), ks.LitString("a")),
					),
				)),
			},
		},
		{
			name: "semantically equal len constraints over a string list",
			cases: []ks.Value{
				// ([(string)], len(x) >= 1, len(x) <= 100)
				ks.With(ks.List(ks.String()),
					ks.Check(
						ks.Binary(
							ks.Funcall("len", ks.X()),
							ks.Ge,
							ks.IntExpr(1),
						),
					),
					ks.Check(
						ks.Binary(
							ks.Funcall("len", ks.X()),
							ks.Le,
							ks.IntExpr(100),
						),
					),
				),
				// ([(string)], len(x) > 0, len(x) <= 100)
				ks.With(ks.List(ks.String()),
					ks.Check(
						ks.Binary(
							ks.Funcall("len", ks.X()),
							ks.Gt,
							ks.IntExpr(0),
						),
					),
					ks.Check(
						ks.Binary(
							ks.Funcall("len", ks.X()),
							ks.Le,
							ks.IntExpr(100),
						),
					),
				),
				// ([(string)], len(x) > 0, len(x) < 101)
				ks.With(ks.List(ks.String()),
					ks.Check(
						ks.Binary(
							ks.Funcall("len", ks.X()),
							ks.Gt,
							ks.IntExpr(0),
						),
					),
					ks.Check(
						ks.Binary(
							ks.Funcall("len", ks.X()),
							ks.Lt,
							ks.IntExpr(101),
						),
					),
				),
			},
		},
		{
			name: "semantically equal len constraints over a object list",
			cases: []ks.Value{
				// ([{a: (int)}], len(x) >= 1, len(x) <= 100)
				ks.With(ks.List(ks.Object(ks.Field("a", ks.Int()))),
					ks.Check(
						ks.Binary(
							ks.Funcall("len", ks.X()),
							ks.Ge,
							ks.IntExpr(1),
						),
					),
					ks.Check(
						ks.Binary(
							ks.Funcall("len", ks.X()),
							ks.Le,
							ks.IntExpr(100),
						),
					),
				),
				// ([(string)], len(x) > 0, len(x) <= 100)
				ks.With(ks.List(ks.Object(ks.Field("a", ks.Int()))),
					ks.Check(
						ks.Binary(
							ks.Funcall("len", ks.X()),
							ks.Gt,
							ks.IntExpr(0),
						),
					),
					ks.Check(
						ks.Binary(
							ks.Funcall("len", ks.X()),
							ks.Le,
							ks.IntExpr(100),
						),
					),
				),
				// ([(string)], len(x) > 0, len(x) < 101)
				ks.With(ks.List(ks.Object(ks.Field("a", ks.Int()))),
					ks.Check(
						ks.Binary(
							ks.Funcall("len", ks.X()),
							ks.Gt,
							ks.IntExpr(0),
						),
					),
					ks.Check(
						ks.Binary(
							ks.Funcall("len", ks.X()),
							ks.Lt,
							ks.IntExpr(101),
						),
					),
				),
			},
		},
		{
			// NOTE(i4k): I'm still not sure of the benefits of len() constraint
			// in objects but allowing in all collections because it seems useful.
			name: "semantically equal len constraints over object",
			cases: []ks.Value{
				// ({a: (int)}, len(x) >= 1, len(x) <= 100)
				ks.With(ks.Object(ks.Field("a", ks.Int())),
					ks.Check(
						ks.Binary(
							ks.Funcall("len", ks.X()),
							ks.Ge,
							ks.IntExpr(1),
						),
					),
					ks.Check(
						ks.Binary(
							ks.Funcall("len", ks.X()),
							ks.Le,
							ks.IntExpr(100),
						),
					),
				),
				// ({a: (int)}, len(x) > 0, len(x) <= 100)
				ks.With(ks.Object(ks.Field("a", ks.Int())),
					ks.Check(
						ks.Binary(
							ks.Funcall("len", ks.X()),
							ks.Gt,
							ks.IntExpr(0),
						),
					),
					ks.Check(
						ks.Binary(
							ks.Funcall("len", ks.X()),
							ks.Le,
							ks.IntExpr(100),
						),
					),
				),
				// ({a: (int)}, len(x) > 0, len(x) < 101)
				ks.With(ks.Object(ks.Field("a", ks.Int())),
					ks.Check(
						ks.Binary(
							ks.Funcall("len", ks.X()),
							ks.Gt,
							ks.IntExpr(0),
						),
					),
					ks.Check(
						ks.Binary(
							ks.Funcall("len", ks.X()),
							ks.Lt,
							ks.IntExpr(101),
						),
					),
				),
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wantID := compile(t, a, tc.cases[0])
			for i, v := range tc.cases[1:] {
				id := compile(t, a, v)
				if id != wantID {
					t.Fatalf("cases[%d]: normalized ID = %d, want %d", i, id, wantID)
				}
			}
		})
	}
}

func TestOptionalObjectOrdering(t *testing.T) {
	t.Parallel()

	// NOTE(i4k): objects with same shape are ordered by their optional fields.
	// This is a very important property for the lattice ordering of types and
	// breaking this would make a lot of things silently fail.

	a := NewArena()
	required := compile(t, a, ks.Object(
		ks.Field("a", ks.String()),
	))
	optional := compile(t, a, ks.Object(
		ks.Field("a", ks.Optional(ks.String())),
	))

	// NOTE(i4k): below we make sure the compiled *optional* and *required* have
	// the expected shape and more importantly they have the optional flag set accordingly!

	fields := a.Type(optional).Fields()
	if fields.Len() != 1 || fields.At(0).Name() != "a" || !fields.At(0).Optional() {
		t.Fatalf("unexpected field view: len=%d name=%q optional=%v", fields.Len(), fields.At(0).Name(), fields.At(0).Optional())
	}

	fields = a.Type(required).Fields()
	if fields.Len() != 1 || fields.At(0).Name() != "a" || fields.At(0).Optional() {
		t.Fatalf("unexpected field view: len=%d name=%q optional=%v", fields.Len(), fields.At(0).Name(), fields.At(0).Optional())
	}

	if required == optional {
		t.Fatal("required and optional objects must be distinct types")
	}
	if !a.Subtype(required, optional) {
		t.Fatal("required object must be <= optional object")
	}
	if a.Subtype(optional, required) {
		t.Fatal("optional object must not be <= required object")
	}
}

func TestTraversalDeterministic(t *testing.T) {
	t.Parallel()

	a := NewArena()
	root := compile(t, a, ks.Object(
		ks.Field("title", ks.String()),
		ks.Field("authors", ks.List(ks.String())),
	))

	obj := a.Type(root)
	if obj.Kind() != Object || obj.Fields().Len() != 2 {
		t.Fatalf("root = %s, fields = %d", obj.Kind(), obj.Fields().Len())
	}

	// Fields are canonicalized by name (lexicographically sorted),
	// making traversal and lookup deterministic.

	authors := obj.Fields().At(0)
	title := obj.Fields().At(1)
	if title.Name() != "title" || a.Type(title.Type()).Kind() != String {
		t.Fatalf("first field = %q/%s", title.Name(), a.Type(title.Type()).Kind())
	}
	if authors.Name() != "authors" {
		t.Fatalf("second field = %q", authors.Name())
	}
	list := a.Type(authors.Type())
	if list.Kind() != List || a.Type(list.Element()).Kind() != String {
		t.Fatalf("tags = %s[%s]", list.Kind(), a.Type(list.Element()).Kind())
	}
}

func TestFingerprintDeterministic(t *testing.T) {
	t.Parallel()

	xa, ya := NewArena(), NewArena()
	x := compile(t, xa, ks.Object(
		ks.Field("z", ks.String()),
		ks.Field("a", ks.Int()),
	))
	y := compile(t, ya, ks.Object(
		ks.Field("a", ks.Int()),
		ks.Field("z", ks.String()),
	))
	if xa.Fingerprint(x) != ya.Fingerprint(y) {
		t.Fatalf("fingerprints differ: %x != %x", xa.Fingerprint(x), ya.Fingerprint(y))
	}
}

func TestHashCollisionDoesNotAliasTypes(t *testing.T) {
	t.Parallel()

	a := newArena(func([]byte) uint64 { return 1 })
	i := compile(t, a, ks.Int())
	s := compile(t, a, ks.String())
	i2 := compile(t, a, ks.Int())
	if i == s {
		t.Fatalf("int and string aliased under hash collision: %d", i)
	}
	if i != i2 {
		t.Fatalf("equal int was not interned: %d != %d", i, i2)
	}
}

// compile into an implicit ast arena. Useful if only one value needs to be compiled.
// Use compileOn() if you need to compile multiple values otherwise they will use
// different arenas.
func compile(t *testing.T, a *Arena, v ks.Value) TypeID {
	t.Helper()
	tree := ast.New()
	return compileOn(t, a, tree, v)
}

func compileOn(t *testing.T, a *Arena, tree *ast.Tree, v ks.Value) TypeID {
	t.Helper()
	root := buildOn(t, tree, v)
	id, err := Compile(a, tree, root)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func build(t *testing.T, v ks.Value) (*ast.Tree, ast.NodeID) {
	t.Helper()
	tree := ast.New()
	root := buildOn(t, tree, v)
	return tree, root
}

func buildOn(t *testing.T, tree *ast.Tree, v ks.Value) ast.NodeID {
	t.Helper()
	root, err := ks.Emit(tree, v)
	if err != nil {
		t.Fatal(err)
	}
	return root
}
