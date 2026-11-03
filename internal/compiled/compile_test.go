package compiled

import (
	"testing"

	"github.com/birdie-ai/katschema/ks"
	"github.com/birdie-ai/katschema/parser/ast"
	"github.com/birdie-ai/katschema/parser/token"
)

func TestCompileIntern(t *testing.T) {
	t.Parallel()
	type testcase struct {
		name         string
		x, y         string
		checkLiteral bool
		wantLiteral  bool
	}
	for _, tc := range []testcase{
		{name: "same builtin type", x: "(int)", y: "(int)", checkLiteral: true},
		{
			name:         "literal int and explicit equality schema",
			x:            "1337",
			y:            "(int, x == 1337)",
			checkLiteral: true,
			wantLiteral:  true,
		},
		{name: "same literal float different syntax", x: "1.01", y: "1.01e0"},
		{
			name:         "literal decimal and explicit equality schema",
			x:            "1.01",
			y:            "(real, x == 1.01)",
			checkLiteral: true,
			wantLiteral:  true,
		},
		{
			name:         "literal bool and explicit equality schema",
			x:            "true",
			y:            "(bool, x == true)",
			checkLiteral: true,
			wantLiteral:  true,
		},
		{
			name:         "literal string and explicit equality schema",
			x:            `"hello"`,
			y:            `(string, x == "hello")`,
			checkLiteral: true,
			wantLiteral:  true,
		},
		{
			name: "object order is ignored",
			x:    `{"id": 1, "id2": 2}`,
			y:    `{"id2": 2, "id": 1}`,
		},
		{name: "A | A => A", x: "(string) | (string)", y: "(string)"},
		{name: "A | (never) => A", x: "(string) | (never)", y: "(string)"},
		{name: "A | (any) => (any)", x: "(string) | (any)", y: "(any)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			arena := NewArena()
			xid := compileText(t, arena, tc.x)
			yid := compileText(t, arena, tc.y)
			if xid != yid {
				t.Fatalf("IDs differ: %d != %d", xid, yid)
			}
			if tc.checkLiteral {
				_, gotLiteral := arena.Literal(xid)
				if gotLiteral != tc.wantLiteral {
					t.Fatalf("Literal(%d) = %t, want %t", xid, gotLiteral, tc.wantLiteral)
				}
				if tc.wantLiteral && arena.Node(xid).Kind() != Refined {
					t.Fatalf("literal kind = %s, want %s", arena.Node(xid).Kind(), Refined)
				}
			}
		})
	}
}

func TestCompileInternAST(t *testing.T) {
	t.Parallel()

	type testcase struct {
		name         string
		x            func(*ast.Tree) ast.NodeID
		y            func(*ast.Tree) ast.NodeID
		checkLiteral bool
		wantLiteral  bool
	}

	z := token.UnknownSpan()

	for _, tc := range []testcase{
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
			name: "same big int literal",
			x: func(a *ast.Tree) ast.NodeID {
				return a.AddInt(google, z)
			},
			y: func(a *ast.Tree) ast.NodeID {
				return a.AddInt("+"+google, z)
			},
		},
		{
			name:         "literal null and explicit equality schema",
			checkLiteral: true,
			wantLiteral:  true,
			x: func(a *ast.Tree) ast.NodeID {
				return a.AddNull(z)
			},
			y: func(a *ast.Tree) ast.NodeID {
				return a.AddSchema(
					a.AddName("null", z),
					[]ast.NodeID{a.AddConstraint(a.AddBinary(
						a.AddIdent("x", z), token.Eq, a.AddNull(z), z), z)},
					z,
				)
			},
		},
		{
			name: "A | A | A => A",
			x: func(a *ast.Tree) ast.NodeID {
				return a.AddSum(
					a.AddSchema(a.AddName("string", z), nil, z),
					a.AddSum(
						a.AddSchema(a.AddName("string", z), nil, z),
						a.AddSchema(a.AddName("string", z), nil, z),
						z,
					),
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
			name: "A | (never) | B => A | B",
			x: func(a *ast.Tree) ast.NodeID {
				return a.AddSum(
					a.AddSchema(a.AddName("string", z), nil, z),
					a.AddSum(
						a.AddSchema(a.AddName("never", z), nil, z),
						a.AddSchema(a.AddName("int", z), nil, z),
						z,
					),
					z,
				)
			},
			y: func(a *ast.Tree) ast.NodeID {
				return a.AddSum(
					a.AddSchema(a.AddName("string", z), nil, z),
					a.AddSchema(a.AddName("int", z), nil, z),
					z,
				)
			},
		},
		{
			name: "A | (any) | B => (any)",
			x: func(a *ast.Tree) ast.NodeID {
				return a.AddSum(
					a.AddSchema(a.AddName("string", z), nil, z),
					a.AddSum(
						a.AddSchema(a.AddName("any", z), nil, z),
						a.AddSchema(a.AddName("int", z), nil, z),
						z,
					),
					z,
				)
			},
			y: func(a *ast.Tree) ast.NodeID {
				return a.AddSchema(a.AddName("any", z), nil, z)
			},
		},
		{
			name: "(any) | A | (any) | B => (any)",
			x: func(a *ast.Tree) ast.NodeID {
				return a.AddSum(
					a.AddSchema(a.AddName("any", z), nil, z),
					a.AddSum(
						a.AddSchema(a.AddName("int", z), nil, z),
						a.AddSum(
							a.AddSchema(a.AddName("any", z), nil, z),
							a.AddSchema(a.AddName("string", z), nil, z),
							z,
						),
						z,
					),
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
			if tc.checkLiteral {
				_, gotLiteral := arena.Literal(xid)
				if gotLiteral != tc.wantLiteral {
					t.Fatalf("Literal(%d) = %t, want %t", xid, gotLiteral, tc.wantLiteral)
				}
				if tc.wantLiteral && arena.Node(xid).Kind() != Refined {
					t.Fatalf("literal kind = %s, want %s", arena.Node(xid).Kind(), Refined)
				}
			}
		})
	}
}

func TestConstraintNormalization(t *testing.T) {
	t.Parallel()
	type testcase struct {
		name  string
		cases []string
	}
	a := NewArena()
	for _, tc := range []testcase{
		{
			name: "semantically equal int ranges",
			cases: []string{
				"(int, x >= 0, x <= 100)",
				"(int, x >= 0 && x <= 100)",
				"(int, 0 <= x <= 100)",
				"(int, -1 < x <= 100)",
				"(int, -1 < x < 101)",
			},
		},
		{
			name: "semantically equal float ranges",
			cases: []string{
				"(float, x >= 0.0, x <= 100.0)",
				"(float, x >= 0.0 && x <= 100.0)",
				"(float, 0.0 <= x <= 100.0)",
			},
		},
		{
			name: "semantically equal enums",
			cases: []string{
				`(string, x in ["a", "b", "c"])`,
				`(string, x in ["b", "a", "c"])`,
				`(string, x in ["c", "b", "a"])`,
				`(string, x in ["c", "b", "a"])`,
				`(string, x == "a" || x == "b" || x == "c")`,
				`(string, x in ["c", "b", "a"])`,
			},
		},
		{
			name: "semantically equal len constraints over a string list",
			cases: []string{
				"([(string)], len(x) >= 1, len(x) <= 100)",
				"([(string)], len(x) > 0, len(x) <= 100)",
				"([(string)], len(x) > 0, len(x) < 101)",
			},
		},
		{
			name: "semantically equal len constraints over an object list",
			cases: []string{
				`([{"a": (int)}], len(x) >= 1, len(x) <= 100)`,
				`([{"a": (int)}], len(x) > 0, len(x) <= 100)`,
				`([{"a": (int)}], len(x) > 0, len(x) < 101)`,
			},
		},
		{
			name: "semantically equal len constraints over object",
			cases: []string{
				`({"a": (int)}, len(x) >= 1, len(x) <= 100)`,
				`({"a": (int)}, len(x) > 0, len(x) <= 100)`,
				`({"a": (int)}, len(x) > 0, len(x) < 101)`,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wantID := compileText(t, a, tc.cases[0])
			for _, src := range tc.cases[1:] {
				id := compileText(t, a, src)
				if id != wantID {
					t.Fatalf("%s: normalized ID = %d, want %d", src, id, wantID)
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
	required := compileText(t, a, `{"a": (string)}`)
	optional := compileText(t, a, `{"a": (string, optional)}`)

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
	root := compileText(t, a, `{"title": (string), "authors": [(string)]}`)

	obj := a.Type(root)
	if obj.Kind() != Object || obj.Fields().Len() != 2 {
		t.Fatalf("root = %s, fields = %d", obj.Kind(), obj.Fields().Len())
	}

	// Fields are canonicalized by name (lexicographically sorted),
	// making traversal and lookup deterministic.

	authors := obj.Fields().At(0)
	title := obj.Fields().At(1)
	if title.Name() != "title" || title.Type().Kind() != String {
		t.Fatalf("first field = %q/%s", title.Name(), title.Type().Kind())
	}
	if authors.Name() != "authors" {
		t.Fatalf("second field = %q", authors.Name())
	}
	list := authors.Type()
	if list.Kind() != List || list.Element().Kind() != String {
		t.Fatalf("tags = %s[%s]", list.Kind(), list.Element().Kind())
	}
}

func TestFingerprintDeterministic(t *testing.T) {
	t.Parallel()

	xa, ya := NewArena(), NewArena()
	x := compileText(t, xa, `{"z": (string), "a": (int)}`)
	y := compileText(t, ya, `{"a": (int), "z": (string)}`)
	if xa.Fingerprint(x) != ya.Fingerprint(y) {
		t.Fatalf("fingerprints differ: %x != %x", xa.Fingerprint(x), ya.Fingerprint(y))
	}
}

func TestHashCollisionDoesNotAliasTypes(t *testing.T) {
	t.Parallel()

	a := newArena(func([]byte) uint64 { return 1 })
	i := compileText(t, a, "(int)")
	s := compileText(t, a, "(string)")
	i2 := compileText(t, a, "(int)")
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
