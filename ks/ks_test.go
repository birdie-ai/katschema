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
				Field("id", With(String(), Flag("pk"))),
				Field("organization_id", With(Int(), Flag("pk"))),
				Field("text", Type("analyzed")),
				Field("kind", With(
					String(),
					Check(
						Binary(X(), In, ListExpr(
							LitString("support_ticket"),
							LitString("nps"),
							LitString("complaint"),
							LitString("etc"),
						)),
					))),
				Field("rating", Optional(Where(Int(),
					Binary(Binary(ValueExpr(LitInt(0)), Le, X()), Le, ValueExpr(LitInt(10))),
				))),
			),
			want: `{"id":(string,pk),"organization_id":(int,pk),"text":(analyzed),"kind":(string,x in ["support_ticket","nps","complaint","etc"]),"rating":(int,0<=x<=10,optional)}`,
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
			name: "constraint with syntactic group",
			root: Where(Int(),
				Binary(
					Binary(X(), Add, ValueExpr(LitInt(1))),
					Lt,
					ValueExpr(LitInt(100)),
				),
			),
			want: `(int,x+1<100)`,
		},
		{
			name: "constraint with explicit group",
			root: Where(Int(),
				Binary(
					Group(Binary(X(), Add, ValueExpr(LitInt(1)))),
					Lt,
					ValueExpr(LitInt(100)),
				),
			),
			want: `(int,(x+1)<100)`,
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

func BenchmarkBuild(b *testing.B) {
	obj := Object(
		Field("id", With(String(), Flag("pk"))),
		Field("organization_id", With(Int(), Flag("pk"))),
		Field("text", Type("analyzed")),
		Field("kind", With(
			String(),
			Check(
				Binary(X(), In, ListExpr(
					LitString("support_ticket"),
					LitString("nps"),
					LitString("complaint"),
					LitString("etc"),
				)),
			))),
		Field("rating", Optional(Where(Int(),
			Binary(Binary(ValueExpr(LitInt(0)), Le, X()), Le, ValueExpr(LitInt(10))),
		))),
	)
	for b.Loop() {
		_, _, err := Build(obj)
		if err != nil {
			b.Fatal(err)
		}
	}
}
