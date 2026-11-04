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
		name    string
		root    Value
		want    string
		wantErr string
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
			name: "empty tuple",
			root: Tuple(),
			want: `#{}`,
		},
		{
			name: "tuple schema",
			root: With(Tuple(String(), Sum(Int(), Bool())), Attr("example", ValueExpr(Tuple(LitString("id"), LitInt(42))))),
			want: `(#{(string),(int)|(bool)},example=#{"id",42})`,
		},
		{
			name: "sum with tuple alternative",
			root: Sum(Tuple(Int()), String()),
			want: `(#{(int)})|(string)`,
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
			name:    "With() rejects scalar type references",
			root:    With(LitBool(true), Flag("optional")),
			wantErr: "ks: invalid type reference",
		},
		{
			name: "With() append clauses to existent schema",
			root: With(String(), Flag("optional")),
			want: `(string,optional)`,
		},
		{
			name: "simple sum type",
			root: Sum(Int(), String()),
			want: `(int)|(string)`,
		},
		{
			name: "sum of types",
			root: Sum(Int(), String(), Float(), Real()),
			want: `(int)|(string)|(float)|(real)`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tree, root, err := Build(tc.root)
			if tc.wantErr != "" {
				if err == nil || err.Error() != tc.wantErr {
					t.Fatalf("Build() error = %v, want %q", err, tc.wantErr)
				}
				return
			}
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

func TestTupleSumBranch(t *testing.T) {
	t.Parallel()
	tree, root, err := Build(Sum(Tuple(Int()), String()))
	if err != nil {
		t.Fatal(err)
	}
	branch := tree.Sum(root).Left
	if tree.Node(branch).Kind() != ast.Schema || tree.Node(tree.Schema(branch).Type).Kind() != ast.Tuple {
		t.Fatal("expected a parenthesized tuple branch")
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
