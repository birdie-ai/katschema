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
	zero := ks.IntExpr(ks.LitInt(0))
	hundred := ks.IntExpr(ks.LitInt(100))

	for _, tc := range []testcase{
		{
			name: "int ranges",
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
			},
		},
		{
			name: "unordered enums",
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

func TestValidation(t *testing.T) {
	t.Parallel()

	type testcase struct {
		name   string
		schema ks.Value
		value  ks.Value
		want   bool
	}

	// (int, 0 <= x <= 100)
	ranged := ks.Where(ks.Int(), ks.Binary(
		ks.Binary(ks.IntExpr(ks.LitInt(0)), ks.Le, ks.X()),
		ks.Le,
		ks.IntExpr(ks.LitInt(100)),
	))

	for _, tc := range []testcase{
		{
			name:   "string: literal different value",
			schema: ks.LitString("expected exact string"),
			value:  ks.LitString("different string"),
			want:   false,
		},
		{
			name:   "string: literal exact value",
			schema: ks.LitString("expected exact string"),
			value:  ks.LitString("expected exact string"),
			want:   true,
		},
		{
			name:   "int: literal exact value",
			schema: ks.LitInt(1337),
			value:  ks.LitInt(1337),
			want:   true,
		},
		{
			name:   "int: literal diferent value",
			schema: ks.LitInt(1337),
			value:  ks.LitInt(1338),
			want:   false,
		},
		{
			name:   "bool: literal exact value",
			schema: ks.LitBool(true),
			value:  ks.LitBool(true),
			want:   true,
		},
		{
			name:   "bool: literal different value",
			schema: ks.LitBool(true),
			value:  ks.LitBool(false),
			want:   false,
		},
		{
			name:   "null: literal different value",
			schema: ks.LitNull(),
			value:  ks.LitBool(true),
			want:   false,
		},
		{
			name:   "null: literal exact value",
			schema: ks.LitNull(),
			value:  ks.LitNull(),
			want:   true,
		},
		{
			name:   "list: literal exact values",
			schema: ks.List(ks.LitString("a")),
			value:  ks.List(ks.LitString("a")),
			want:   true,
		},
		{
			name:   "list: literal different values",
			schema: ks.List(ks.LitString("a")),
			value:  ks.List(ks.LitString("b")),
			want:   false,
		},
		{
			name:   "list: literal type does not accept empty list",
			schema: ks.List(ks.LitString("a")),
			value:  ks.List(),
			want:   false,
		},
		{
			name:   "different type",
			schema: ks.String(),
			value:  ks.LitInt(10),
			want:   false,
		},
		{
			name:   "(int) against literal int",
			schema: ks.Int(),
			value:  ks.LitInt(67),
			want:   true,
		},
		{
			name:   "(int) rejects literal float",
			schema: ks.Int(),
			value:  ks.LitFloat(69.0),
			want:   false,
		},

		// NOTE(i4k): BETTER SAFE THAN SORRY!
		// Finally solving that subtle bug when use large integers in JSON :-)
		{

			name:   "(float) accepts integers that can be representable in float without loss of precision",
			schema: ks.Float(),
			value:  ks.LitInt(int64(2<<53) + 0),
			want:   true,
		},
		{

			name:   "(float) rejects integers that cannot be representable in float",
			schema: ks.Float(),
			value:  ks.LitInt(int64(2<<53) + 1),
			want:   false,
		},

		{
			name:   "valid on the lower bound edge",
			schema: ranged,
			value:  ks.LitInt(0),
			want:   true,
		},
		{
			name:   "valid on the upper bound edge",
			schema: ranged,
			value:  ks.LitInt(100),
			want:   true,
		},
		{
			name:   "valid somewhere in the middle",
			schema: ranged,
			value:  ks.LitInt(67),
			want:   true,
		},
		{
			name:   "outside range is rejected",
			schema: ranged,
			value:  ks.LitInt(1337),
			want:   false,
		},
		{
			name: "int: value has valid in enum",
			schema: ks.With(
				ks.Int(),
				ks.Check(
					ks.Binary(
						ks.X(),
						ks.In,
						ks.ListExpr(ks.LitInt(0), ks.LitInt(1), ks.LitInt(2)),
					),
				),
			),
			value: ks.LitInt(1),
			want:  true,
		},
		{
			name: "int: value has valid not in enum",
			schema: ks.With(
				ks.Int(),
				ks.Check(
					ks.Binary(
						ks.X(),
						ks.In,
						ks.ListExpr(ks.LitInt(0), ks.LitInt(1), ks.LitInt(2)),
					),
				),
			),
			value: ks.LitInt(3),
			want:  false,
		},
		{
			name:   "required field with provided value",
			schema: ks.Object(ks.Field("a", ks.String())),
			value:  ks.Object(ks.Field("a", ks.LitString("xxx"))),
			want:   true,
		},
		{
			name:   "required field with absent value",
			schema: ks.Object(ks.Field("a", ks.String())),
			value:  ks.Object(),
			want:   false,
		},
		{
			name: "optional field missing is fine",
			schema: ks.Object(
				ks.Field("a", ks.String()),
				ks.Field("b", ks.Optional(ks.Int())),
			),
			value: ks.Object(ks.Field("a", ks.LitString("Triple X"))),
			want:  true,
		},
		{
			name: "optional field is an object with required fields",
			schema: ks.Object(
				ks.Field("a", ks.String()),
				ks.Field("b", ks.Optional(ks.Object(
					ks.Field("a", ks.String()),
					ks.Field("b", ks.Int()),
				))),
			),
			value: ks.Object(ks.Field("a", ks.LitString("Triple X"))),
			want:  true,
		},
		{
			name:   "value has extra field",
			schema: ks.Object(ks.Field("a", ks.String())),
			value: ks.Object(
				ks.Field("a", ks.LitString("xxx")),
				ks.Field("b", ks.LitString("yyy")),
			),
			want: false,
		},
		{
			name: "validation recurses in object fields : missing b.b",
			schema: ks.Object(
				ks.Field("a", ks.String()),
				ks.Field("b", ks.Optional(ks.Object(
					ks.Field("a", ks.String()),
					ks.Field("b", ks.Int()),
				))),
			),
			value: ks.Object(
				ks.Field("a", ks.LitString("xXx")),
				ks.Field("b", ks.Object(
					ks.Field("a", ks.LitString("Xxx")),
				)),
			),
			want: false,
		},
		{
			name: "validation recurses in object fields : all keys provided",
			schema: ks.Object(
				ks.Field("a", ks.String()),
				ks.Field("b", ks.Optional(ks.Object(
					ks.Field("a", ks.String()),
					ks.Field("b", ks.Int()),
				))),
			),
			value: ks.Object(
				ks.Field("a", ks.LitString("xXx")),
				ks.Field("b", ks.Object(
					ks.Field("a", ks.LitString("Xxx")),
					ks.Field("b", ks.LitInt(1337)),
				)),
			),
			want: true,
		},
		{
			name:   "[(string)] accepts []",
			schema: ks.List(ks.String()),
			value:  ks.List(),
			want:   true,
		},
		{
			name:   "[(string)] accepts [... strings ...]",
			schema: ks.List(ks.String()),
			value:  ks.List(ks.LitString("kat"), ks.LitString("schema")),
			want:   true,
		},
		{
			name:   "[(string)] rejects [... integers ...]",
			schema: ks.List(ks.String()),
			value:  ks.List(ks.LitInt(67), ks.LitInt(69)),
			want:   false,
		},
		{
			name:   "[(string)] rejects [... mixed types ...]",
			schema: ks.List(ks.String()),
			value:  ks.List(ks.LitString("valid"), ks.LitInt(67)),
			want:   false,
		},
		{
			name:   "[(int, x > 0)] accepts []",
			schema: ks.List(ks.With(ks.Int(), ks.Check(ks.Binary(ks.X(), ks.Gt, ks.ValueExpr(ks.LitInt(0)))))),
			value:  ks.List(),
			want:   true,
		},
		{
			name:   "[(int, x > 0)] accepts [1, 2, 3]",
			schema: ks.List(ks.With(ks.Int(), ks.Check(ks.Binary(ks.X(), ks.Gt, ks.ValueExpr(ks.LitInt(0)))))),
			value:  ks.List(ks.LitInt(1), ks.LitInt(2), ks.LitInt(3)),
			want:   true,
		},
		{
			name:   "[(int, x > 0)] rejects [1, 2, 3, -1]",
			schema: ks.List(ks.With(ks.Int(), ks.Check(ks.Binary(ks.X(), ks.Gt, ks.ValueExpr(ks.LitInt(0)))))),
			value:  ks.List(ks.LitInt(1), ks.LitInt(2), ks.LitInt(3), ks.LitInt(-1)),
			want:   false,
		},
		{
			name: "([(int)], len(x)>2) accepts [1, 2, 3]",
			schema: ks.With(
				ks.List(ks.Int()),
				ks.Check(
					ks.Binary(
						ks.Call("len", ks.X()),
						ks.Gt,
						ks.ValueExpr(ks.LitInt(2)),
					),
				),
			),
			value: ks.List(ks.LitInt(1), ks.LitInt(2), ks.LitInt(3)),
			want:  true,
		},
		{
			name: "([(int)], len(x)>2) rejects [1, 2]",
			schema: ks.With(
				ks.List(ks.Int()),
				ks.Check(
					ks.Binary(
						ks.Call("len", ks.X()),
						ks.Gt,
						ks.ValueExpr(ks.LitInt(2)),
					),
				),
			),
			value: ks.List(ks.LitInt(1), ks.LitInt(2)),
			want:  false,
		},
		{
			name: "([(int, x < 0)], len(x)>2) accepts [-1, -2, -3]",
			schema: ks.With(
				ks.List(
					ks.With(
						ks.Int(),
						ks.Check(
							ks.Binary(ks.X(), ks.Lt, ks.ValueExpr(ks.LitInt(0))),
						),
					),
				),
				ks.Check(
					ks.Binary(
						ks.Call("len", ks.X()),
						ks.Gt,
						ks.ValueExpr(ks.LitInt(2)),
					),
				),
			),
			value: ks.List(ks.LitInt(-1), ks.LitInt(-2), ks.LitInt(-3)),
			want:  true,
		},
		{
			name: "([(int, x < 0)], len(x)>2) rejects [-1, -2, 1]",
			schema: ks.With(
				ks.List(
					ks.With(
						ks.Int(),
						ks.Check(
							ks.Binary(ks.X(), ks.Lt, ks.ValueExpr(ks.LitInt(0))),
						),
					),
				),
				ks.Check(
					ks.Binary(
						ks.Call("len", ks.X()),
						ks.Gt,
						ks.ValueExpr(ks.LitInt(2)),
					),
				),
			),
			value: ks.List(ks.LitInt(-1), ks.LitInt(-2), ks.LitInt(1)),
			want:  false,
		},
		{
			name: "([(int)], len(x)>2) rejects [1, 2]",
			schema: ks.With(
				ks.List(ks.Int()),
				ks.Check(
					ks.Binary(
						ks.Call("len", ks.X()),
						ks.Gt,
						ks.ValueExpr(ks.LitInt(2)),
					),
				),
			),
			value: ks.List(ks.LitInt(1), ks.LitInt(2)),
			want:  false,
		},
		{
			name:   "[(int, x > 0)] rejects [1, 2, 3, -1]",
			schema: ks.List(ks.With(ks.Int(), ks.Check(ks.Binary(ks.X(), ks.Gt, ks.ValueExpr(ks.LitInt(0)))))),
			value:  ks.List(ks.LitInt(1), ks.LitInt(2), ks.LitInt(3), ks.LitInt(-1)),
			want:   false,
		},
		{
			name: "[(int)]: value has valid in enum",
			schema: ks.List(ks.With(
				ks.Int(),
				ks.Check(
					ks.Binary(
						ks.X(),
						ks.In,
						ks.ListExpr(ks.LitInt(0), ks.LitInt(1), ks.LitInt(2)),
					),
				),
			)),
			value: ks.List(ks.LitInt(1)),
			want:  true,
		},
		{
			name: "[(int)]: value not in enum",
			schema: ks.List(ks.With(
				ks.Int(),
				ks.Check(
					ks.Binary(
						ks.X(),
						ks.In,
						ks.ListExpr(ks.LitInt(0), ks.LitInt(1), ks.LitInt(2)),
					),
				),
			)),
			value: ks.List(ks.LitInt(3)),
			want:  false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := NewArena()
			typ := compile(t, a, tc.schema)
			values, root := build(t, tc.value)
			if got := a.Valid(typ, values, root); got != tc.want {
				t.Fatalf("Valid = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestTraversal(t *testing.T) {
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

func compile(t *testing.T, a *Arena, v ks.Value) TypeID {
	t.Helper()
	tree, root, err := ks.Build(v)
	if err != nil {
		t.Fatal(err)
	}
	id, err := Compile(a, tree, root)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func build(t *testing.T, v ks.Value) (*ast.Tree, ast.NodeID) {
	t.Helper()
	tree, root, err := ks.Build(v)
	if err != nil {
		t.Fatal(err)
	}
	return tree, root
}
