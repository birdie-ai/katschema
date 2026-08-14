package ast

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/birdie-ai/katschema/parser/token"
)

func TestPrint(t *testing.T) {
	t.Parallel()

	type testcase struct {
		name string
		root func(*Tree) NodeID
		want string
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
							a.NewField("str", a.AddString("test", z), z, z),
							a.NewField("int", a.AddInt("1337", z), z, z),
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
			name: "schema int binary constraint (x == 67)",
			root: func(a *Tree) NodeID {
				return a.AddSchema(a.AddName("int", z),
					[]NodeID{a.AddConstraint(
						a.AddBinary(
							a.AddIdent("x", z),
							token.Eq,
							a.AddInt("67", z),
							z,
						), z)},
					z)
			},
			want: `(int,x==67)`,
		},
		{
			name: "schema int binary constraint (x != 67)",
			root: func(a *Tree) NodeID {
				return a.AddSchema(a.AddName("int", z),
					[]NodeID{a.AddConstraint(
						a.AddBinary(
							a.AddIdent("x", z),
							token.Ne,
							a.AddInt("67", z),
							z,
						), z)},
					z)
			},
			want: `(int,x!=67)`,
		},
		{
			name: "schema int binary constraint (x < 69)",
			root: func(a *Tree) NodeID {
				return a.AddSchema(a.AddName("int", z),
					[]NodeID{a.AddConstraint(
						a.AddBinary(
							a.AddIdent("x", z),
							token.Lt,
							a.AddInt("69", z),
							z,
						), z)},
					z)
			},
			want: `(int,x<69)`,
		},
		{
			name: "schema int binary constraint (0 <= x <= 1337)",
			root: func(a *Tree) NodeID {
				return a.AddSchema(a.AddName("int", z),
					[]NodeID{a.AddConstraint(a.AddBinary(
						a.AddBinary(
							a.AddInt("0", z),
							token.Le,
							a.AddIdent("x", z),
							z,
						),
						token.Le,
						a.AddInt("1337", z),
						z,
					), z)},
					z)
			},
			want: `(int,0<=x<=1337)`,
		},
		{
			name: "schema int binary constraint (x IN [0, 1])",
			root: func(a *Tree) NodeID {
				return a.AddSchema(a.AddName("int", z),
					[]NodeID{a.AddConstraint(
						a.AddBinary(
							a.AddIdent("x", z),
							token.In,
							a.AddList([]NodeID{a.AddInt("0", z), a.AddInt("1", z)}, z),
							z,
						), z)},
					z)
			},
			want: `(int,x in [0,1])`,
		},
		{
			name: "schema string binary constraint (x ~ \"[a-z]+\")",
			root: func(a *Tree) NodeID {
				return a.AddSchema(a.AddName("string", z),
					[]NodeID{a.AddConstraint(
						a.AddBinary(
							a.AddIdent("x", z),
							token.Match,
							a.AddString("[a-z]+", z),
							z,
						), z)},
					z)
			},
			want: `(string,x~"[a-z]+")`,
		},
		{
			name: "binary constraint with arith",
			root: func(a *Tree) NodeID {
				x := a.AddIdent("x", z)
				one := a.AddFloat("1", z)
				two := a.AddFloat("2", z)

				add := a.AddBinary(x, token.Add, one, z)
				group := a.AddGroup(z, add)
				mul := a.AddBinary(group, token.Mul, two, z)
				return a.AddSchema(a.AddName("int", z),
					[]NodeID{a.AddConstraint(
						a.AddBinary(
							a.AddIdent("x", z),
							token.Lt,
							mul,
							z,
						), z)},
					z)
			},
			want: `(int,x<(x+1)*2)`,
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

func BenchmarkStdlibEncodingJSON(b *testing.B) {
	b.StopTimer()
	obj, a := benchValue()

	// we encode a Katschema value that's JSON compatible and then
	// benchmark encoding/json encoding the decoded value.

	var buf bytes.Buffer
	if err := Print(&buf, a, obj); err != nil {
		b.Fatal(err)
	}

	var m map[string]any
	err := json.Unmarshal(buf.Bytes(), &m)
	if err != nil {
		b.Fatal(err)
	}
	// NOTE(i4k): do not reset *buf* because otherwise encoding/json will be allocation free.
	var buf2 bytes.Buffer
	enc := json.NewEncoder(&buf2)

	b.StartTimer()
	for b.Loop() {
		err := enc.Encode(m)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPrint(b *testing.B) {
	b.StopTimer()
	obj, a := benchValue()
	b.StartTimer()
	for b.Loop() {
		var buf bytes.Buffer
		if err := Print(&buf, a, obj); err != nil {
			b.Fatal(err)
		}
	}
}

func benchValue() (NodeID, *Tree) {
	var z token.Span
	a := New()
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
			Name: a.AddText("obj"),
			Value: a.AddObject([]Field{
				a.NewField("str", a.AddString("test", z), z, z),
				a.NewField("int", a.AddInt("1337", z), z, z),
			}, z),
		},
	}, z), a
}
