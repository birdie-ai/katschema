package ks_test

import (
	"bytes"
	"testing"

	"github.com/birdie-ai/katschema/parser/ast"

	. "github.com/birdie-ai/katschema/ks"
)

func TestBuild(t *testing.T) {
	v := Object(
		Field("id", Type("uuid")),
		Field("name", String()),
		Field("role", Where(String(),
			Binary(X(), In, List(LitString("admin"), LitString("user"), LitString("guest"))),
		)),
		Field("age", Optional(Where(Int(),
			Binary(X(), Ge, Num("18")),
		))),
	)

	tree, root, err := Build(v)
	if err != nil {
		t.Fatal(err)
	}

	var b bytes.Buffer
	if err := ast.Print(&b, tree, root); err != nil {
		t.Fatal(err)
	}

	const want = `{"id":(uuid),"name":(string),"role":(string,x in ["admin","user","guest"]),"age":(int,x>=18,optional)}`
	if got := b.String(); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestExplicitGroup(t *testing.T) {
	v := Where(Int(),
		Binary(
			Group(Binary(X(), Add, Num("1"))),
			Mul,
			Num("2"),
		),
	)

	tree, root, err := Build(v)
	if err != nil {
		t.Fatal(err)
	}

	var b bytes.Buffer
	if err := ast.Print(&b, tree, root); err != nil {
		t.Fatal(err)
	}
	if got, want := b.String(), `(int,(x+1)*2)`; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestObjectTypeRef(t *testing.T) {
	v := Optional(Object(
		Field("street", String()),
		Field("zip", String()),
	))

	tree, root, err := Build(v)
	if err != nil {
		t.Fatal(err)
	}

	var b bytes.Buffer
	if err := ast.Print(&b, tree, root); err != nil {
		t.Fatal(err)
	}
	if got, want := b.String(), `({"street":(string),"zip":(string)},optional)`; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
