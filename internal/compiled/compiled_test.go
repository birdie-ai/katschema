package compiled

import (
	"testing"

	"github.com/birdie-ai/katschema/ks"
	"github.com/birdie-ai/katschema/parser/ast"
)

func compileKS(t *testing.T, a *Arena, v ks.Value) TypeID {
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

func valueKS(t *testing.T, v ks.Value) (*ast.Tree, ast.NodeID) {
	t.Helper()
	tree, root, err := ks.Build(v)
	if err != nil {
		t.Fatal(err)
	}
	return tree, root
}

func TestCompileIntern(t *testing.T) {
	tests := []struct {
		name string
		a    ks.Value
		b    ks.Value
	}{
		{"primitive", ks.Int(), ks.Int()},
		{"literal int", ks.LitInt("10"), ks.LitInt("10")},
		{"number spelling", ks.LitFloat("1.0"), ks.LitFloat("1e0")},
		{
			"object order",
			ks.Object(ks.Field("a", ks.Int()), ks.Field("b", ks.String())),
			ks.Object(ks.Field("b", ks.String()), ks.Field("a", ks.Int())),
		},
		{
			"metadata ignored",
			ks.With(ks.Int(), ks.Attr("pk", ks.Boolean(true))),
			ks.Int(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := NewArena()
			x := compileKS(t, a, tt.a)
			y := compileKS(t, a, tt.b)
			if x != y {
				t.Fatalf("IDs differ: %d != %d", x, y)
			}
		})
	}
}

func TestFingerprintDeterministic(t *testing.T) {
	xa, ya := NewArena(), NewArena()
	x := compileKS(t, xa, ks.Object(
		ks.Field("z", ks.String()),
		ks.Field("a", ks.Int()),
	))
	y := compileKS(t, ya, ks.Object(
		ks.Field("a", ks.Int()),
		ks.Field("z", ks.String()),
	))
	if xa.Fingerprint(x) != ya.Fingerprint(y) {
		t.Fatalf("fingerprints differ: %x != %x", xa.Fingerprint(x), ya.Fingerprint(y))
	}
}

func TestHashCollisionDoesNotAliasTypes(t *testing.T) {
	a := newArena(func([]byte) uint64 { return 1 })
	i := compileKS(t, a, ks.Int())
	s := compileKS(t, a, ks.String())
	i2 := compileKS(t, a, ks.Int())
	if i == s {
		t.Fatalf("int and string aliased under hash collision: %d", i)
	}
	if i != i2 {
		t.Fatalf("equal int was not interned: %d != %d", i, i2)
	}
}

func TestConstraintNormalization(t *testing.T) {
	a := NewArena()
	x := ks.X()
	zero := ks.IntExpr(ks.LitInt("0"))
	hundred := ks.IntExpr(ks.LitInt("100"))

	tests := []struct {
		name string
		v    ks.Value
	}{
		{
			"clauses",
			ks.With(ks.Int(),
				ks.Check(ks.Binary(x, ks.Ge, zero)),
				ks.Check(ks.Binary(x, ks.Le, hundred)),
			),
		},
		{
			"and",
			ks.Where(ks.Int(), ks.Binary(
				ks.Binary(x, ks.Ge, zero),
				ks.And,
				ks.Binary(x, ks.Le, hundred),
			)),
		},
		{
			"chain",
			ks.Where(ks.Int(), ks.Binary(
				ks.Binary(zero, ks.Le, x),
				ks.Le,
				hundred,
			)),
		},
	}

	var first TypeID
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := compileKS(t, a, tt.v)
			if first == 0 {
				first = id
			} else if id != first {
				t.Fatalf("normalized ID = %d, want %d", id, first)
			}
		})
	}
}

func TestOptionalObjectOrdering(t *testing.T) {
	a := NewArena()
	required := compileKS(t, a, ks.Object(
		ks.Field("a", ks.String()),
	))
	optional := compileKS(t, a, ks.Object(
		ks.Field("a", ks.Optional(ks.String())),
	))

	if required == optional {
		t.Fatal("required and optional objects must be distinct types")
	}
	if !a.Subtype(required, optional) {
		t.Fatal("required object must be <= optional object")
	}
	if a.Subtype(optional, required) {
		t.Fatal("optional object must not be <= required object")
	}

	fields := a.Type(optional).Fields()
	if fields.Len() != 1 || fields.At(0).Name() != "a" || !fields.At(0).Optional() {
		t.Fatalf("unexpected field view: len=%d name=%q optional=%v", fields.Len(), fields.At(0).Name(), fields.At(0).Optional())
	}
}

func TestValidation(t *testing.T) {
	ranged := ks.Where(ks.Int(), ks.Binary(
		ks.Binary(ks.IntExpr(ks.LitInt("0")), ks.Le, ks.X()),
		ks.Le,
		ks.IntExpr(ks.LitInt("100")),
	))

	tests := []struct {
		name   string
		schema ks.Value
		value  ks.Value
		want   bool
	}{
		{"int", ks.Int(), ks.LitInt("10"), true},
		{"int rejects float", ks.Int(), ks.LitInt("10.0"), false},
		{"number accepts int", ks.Number(), ks.LitInt("10"), true},
		{"range lower", ranged, ks.LitInt("0"), true},
		{"range upper", ranged, ks.LitInt("100"), true},
		{"range reject", ranged, ks.LitInt("101"), false},
		{
			"required field",
			ks.Object(ks.Field("a", ks.String())),
			ks.Object(ks.Field("a", ks.LitString("x"))),
			true,
		},
		{
			"required missing",
			ks.Object(ks.Field("a", ks.String())),
			ks.Object(),
			false,
		},
		{
			"optional missing",
			ks.Object(ks.Field("a", ks.Optional(ks.String()))),
			ks.Object(),
			true,
		},
		{
			"unknown field",
			ks.Object(ks.Field("a", ks.String())),
			ks.Object(ks.Field("a", ks.LitString("x")), ks.Field("b", ks.LitInt("1"))),
			false,
		},
		{
			"list",
			ks.List(ks.String()),
			ks.List(ks.LitString("a"), ks.LitString("b")),
			true,
		},
		{
			"list reject",
			ks.List(ks.String()),
			ks.List(ks.LitString("a"), ks.LitInt("1")),
			false,
		},
		{
			"literal tuple",
			ks.List(ks.LitInt("1"), ks.LitString("a")),
			ks.List(ks.LitInt("1"), ks.LitString("a")),
			true,
		},
		{
			"literal tuple reject",
			ks.List(ks.LitInt("1"), ks.LitString("a")),
			ks.List(ks.LitInt("1"), ks.LitString("b")),
			false,
		},
		{
			"enum",
			ks.Where(ks.String(), ks.Binary(
				ks.X(), ks.In, ks.ListExpr(ks.LitString("admin"), ks.LitString("user")),
			)),
			ks.LitString("user"),
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := NewArena()
			typ := compileKS(t, a, tt.schema)
			values, root := valueKS(t, tt.value)
			if got := a.Valid(typ, values, root); got != tt.want {
				t.Fatalf("Valid = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTraversal(t *testing.T) {
	a := NewArena()
	root := compileKS(t, a, ks.Object(
		ks.Field("name", ks.String()),
		ks.Field("tags", ks.List(ks.String())),
	))

	obj := a.Type(root)
	if obj.Kind() != Object || obj.Fields().Len() != 2 {
		t.Fatalf("root = %s, fields = %d", obj.Kind(), obj.Fields().Len())
	}

	// Fields are canonicalized by name, making traversal and lookup predictable.
	name := obj.Fields().At(0)
	tags := obj.Fields().At(1)
	if name.Name() != "name" || a.Type(name.Type()).Kind() != String {
		t.Fatalf("first field = %q/%s", name.Name(), a.Type(name.Type()).Kind())
	}
	if tags.Name() != "tags" {
		t.Fatalf("second field = %q", tags.Name())
	}
	list := a.Type(tags.Type())
	if list.Kind() != List || a.Type(list.Element()).Kind() != String {
		t.Fatalf("tags = %s[%s]", list.Kind(), a.Type(list.Element()).Kind())
	}
}
