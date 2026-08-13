package ast

import (
	"bytes"
	"testing"

	"github.com/birdie-ai/katschema/parser/token"
)

func TestPrint(t *testing.T) {
	t.Parallel()

	type testcase struct {
		name string
		root func(*Tree) NodeID
		want string
		err  error
	}

	var z token.Span

	for _, tc := range []testcase{
		{
			name: "null value",
			root: func(a *Tree) NodeID { return a.AddNull(z) },
			want: `null`,
		},
		{
			name: "true value",
			root: func(a *Tree) NodeID { return a.AddBool(true, z) },
			want: `true`,
		},
		{
			name: "false value",
			root: func(a *Tree) NodeID { return a.AddBool(false, z) },
			want: `false`,
		},
		{
			name: "negative integer",
			root: func(a *Tree) NodeID { return a.AddInt("-1337", z) },
			want: `-1337`,
		},
		{
			name: "positive integer",
			root: func(a *Tree) NodeID { return a.AddInt("1337", z) },
			want: `1337`,
		},
		{
			name: "negative float",
			root: func(a *Tree) NodeID { return a.AddFloat("-13.37", z) },
			want: `-13.37`,
		},
		{
			name: "positive float",
			root: func(a *Tree) NodeID { return a.AddFloat("13.37", z) },
			want: `13.37`,
		},
		{
			name: "string",
			root: func(a *Tree) NodeID { return a.AddString("when in doubt, use brute force", z) },
			want: `"when in doubt, use brute force"`,
		},
		{
			name: "array",
			root: func(a *Tree) NodeID {
				return a.AddList([]NodeID{a.AddString("test-1", z), a.AddString("test-2", z)}, z)
			},
			want: `["test-1","test-2"]`,
		},
		{
			name: "empty object",
			root: func(a *Tree) NodeID {
				return a.AddObject([]Field{}, z)
			},
			want: `{}`,
		},
		{
			name: "simple object",
			root: func(a *Tree) NodeID {
				return a.AddObject([]Field{
					{
						Name:  a.AddText("str"),
						Value: a.AddString("test", z),
					},
					{
						Name:  a.AddText("int"),
						Value: a.AddInt("1337", z),
					},
					{
						Name:  a.AddText("null"),
						Value: a.AddNull(z),
					},
					{
						Name:  a.AddText("float"),
						Value: a.AddFloat("-3.141519", z),
					},
					{
						Name:  a.AddText("list"),
						Value: a.AddList([]NodeID{a.AddString("str", z), a.AddInt("1337", z)}, z),
					},
					{
						Name:  a.AddText("type"),
						Value: a.AddSchema(a.AddName("uuid", z), nil, z),
					},
					{
						Name: a.AddText("obj"),
						Value: a.AddObject([]Field{
							{
								Name:  a.AddText("str"),
								Value: a.AddString("test", z),
							},
							{
								Name:  a.AddText("int"),
								Value: a.AddInt("1337", z),
							},
						}, z),
					},
				}, z)
			},
			want: `{"str":"test","int":1337,"null":null,"float":-3.141519,"list":["str",1337],"type":(uuid),"obj":{"str":"test","int":1337}}`,
		},
		{
			name: "schema ref",
			root: func(a *Tree) NodeID {
				return a.AddSchema(a.AddName("uuid", z), nil, z)
			},
			want: `(uuid)`,
		},
		{
			name: "literal integer schema",
			root: func(a *Tree) NodeID {
				return a.AddSchema(a.AddInt("1", z), nil, z)
			},
			want: `(1)`,
		},
		{
			name: "literal null schema",
			root: func(a *Tree) NodeID {
				return a.AddSchema(a.AddNull(z), nil, z)
			},
			want: `(null)`,
		},
		{
			name: "literal float schema",
			root: func(a *Tree) NodeID {
				return a.AddSchema(a.AddFloat("3.14", z), nil, z)
			},
			want: `(3.14)`,
		},
		{
			name: "literal list schema",
			root: func(a *Tree) NodeID {
				return a.AddSchema(a.AddList([]NodeID{a.AddString("test", z)}, z), nil, z)
			},
			want: `(["test"])`,
		},
		{
			name: "literal object schema",
			root: func(a *Tree) NodeID {
				return a.AddSchema(a.AddObject([]Field{}, z), nil, z)
			},
			want: `({})`,
		},
		{
			name: "schema int constraint",
			root: func(a *Tree) NodeID {
				return a.AddSchema(a.AddName("int", z),
					[]NodeID{a.AddConstraint(a.AddBinary(
						a.AddBinary(
							a.AddInt("0", z),
							token.LE,
							a.AddIdent("x", z),
							z,
						),
						token.LE,
						a.AddInt("100", z),
						z,
					), z)},
					z)
			},
			want: `(int,0<=x<=100)`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tree := New()
			root := tc.root(tree)
			var b bytes.Buffer
			if err := Print(&b, tree, root); err != nil {
				t.Fatal(err)
			}
			if got := b.String(); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestPAddFloattExplicit(t *testing.T) {
	tree := New()
	var z token.Span
	x := tree.AddIdent("x", z)
	one := tree.AddFloat("1", z)
	two := tree.AddFloat("2", z)

	add := tree.AddBinary(x, token.ADD, one, z)
	group := tree.AddGroup(z, add)
	mul := tree.AddBinary(group, token.MUL, two, z)

	var b bytes.Buffer
	if err := Print(&b, tree, mul); err != nil {
		t.Fatal(err)
	}
	if got, want := b.String(), `(x+1)*2`; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestAddFloat(t *testing.T) {
	tree := New()
	span := token.NewSpan(10, 14)
	id := tree.AddFloat("1337", span)

	if got := tree.Node(id).Span(); got != span {
		t.Fatalf("got %#v, want %#v", got, span)
	}
	if span.Start.Offset() != 10 || span.End.Offset() != 14 {
		t.Fatalf("bad offsets: %#v", span)
	}
}
