package ks_test

import (
	"bytes"
	"testing"

	"github.com/birdie-ai/katschema/parser/ast"

	. "github.com/birdie-ai/katschema/ks"
)

func TestDSL(t *testing.T) {
	t.Parallel()

	type testcase struct {
		name string
		root Value
		want string
	}

	for _, tc := range []testcase{
		{
			name: "null",
			root: LitNull(),
			want: `null`,
		},
		{
			name: "true",
			root: LitBool(true),
			want: `true`,
		},
		{
			name: "empty list",
			root: List(),
			want: `[]`,
		},
		{
			name: "list",
			root: List(LitNull(), LitBool(false)),
			want: `[null,false]`,
		},
		{
			name: "object",
			root: Object(
				Field("id", Type("uuid")),
				Field("name", String()),
				Field("role", Where(String(),
					Binary(X(), In, ListExpr(LitString("admin"), LitString("user"), LitString("guest"))),
				)),
				Field("age", Optional(Where(Int(),
					Binary(X(), Ge, IntExpr(LitInt("18"))),
				))),
			),
			want: `{"id":(uuid),"name":(string),"role":(string,x in ["admin","user","guest"]),"age":(int,x>=18,optional)}`,
		},
		{
			name: "optional literal object",
			root: Optional(Object(
				Field("street", String()),
				Field("zip", String()),
			)),
			want: `({"street":(string),"zip":(string)},optional)`,
		},
		{
			name: "constraint with groups",
			root: Where(Int(),
				Binary(
					Group(Binary(X(), Add, IntExpr(LitInt("1")))),
					Mul,
					IntExpr(LitInt("2")),
				),
			),
			want: `(int,(x+1)*2)`,
		},
		{
			name: "With() wraps literal values in a schema",
			root: With(LitBool(true), Flag("optional")),
			want: `(true,optional)`,
		},
		{
			name: "With() append clauses to existent schema",
			root: With(String(), Flag("optional")),
			want: `(string,optional)`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tree, root, err := Build(tc.root)
			if err != nil {
				t.Fatal(err)
			}
			var b bytes.Buffer
			if err := ast.Print(&b, tree, root); err != nil {
				t.Fatal(err)
			}
			if got := b.String(); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
