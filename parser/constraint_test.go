package parser

import (
	"errors"
	"testing"

	"github.com/birdie-ai/katschema/internal/compiled"
	"github.com/birdie-ai/katschema/parser/ast"
	"github.com/birdie-ai/katschema/parser/token"
	"github.com/google/go-cmp/cmp"
)

func TestParseExpressionPrecedence(t *testing.T) {
	t.Parallel()
	type testcase struct {
		src     string
		op      token.Kind
		leftOp  token.Kind
		rightOp token.Kind
	}
	for _, tc := range []testcase{
		{src: "a || b && c", op: token.Or, rightOp: token.And},
		{src: "a && b == c", op: token.And, rightOp: token.Eq},
		{src: "a != b < c", op: token.Ne, rightOp: token.Lt},
		{src: "a < b + c", op: token.Lt, rightOp: token.Add},
		{src: "a + b * c", op: token.Add, rightOp: token.Mul},
		{src: "!a * -b", op: token.Mul},
		{src: "a - b - c", op: token.Sub, leftOp: token.Sub},
		{src: "a / b % c", op: token.Mod, leftOp: token.Div},
		{src: "0 <= x <= 100", op: token.Le, leftOp: token.Le},
		{src: "100 > x >= 0", op: token.Ge, leftOp: token.Gt},
		{src: "a || b || c", op: token.Or, leftOp: token.Or},
		{src: "a && b && c", op: token.And, leftOp: token.And},
		{src: "x in [1,2]", op: token.In},
		{src: `x ~ "[a-z]+"`, op: token.Match},
	} {
		t.Run(tc.src, func(t *testing.T) {
			tree, root := parseOK(t, "(int, "+tc.src+")")
			clause := tree.Schema(root).Clauses[0]
			expr := tree.Constraint(clause)
			if tree.Node(clause).Kind() != ast.Constraint || tree.Node(expr).Kind() != ast.Binary {
				t.Fatal("expected a binary constraint")
			}
			binary := tree.Binary(expr)
			got := []token.Kind{binary.Op, tree.Binary(binary.Left).Op, tree.Binary(binary.Right).Op}
			want := []token.Kind{tc.op, tc.leftOp, tc.rightOp}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Fatalf("operator tree differs (-want +got):\n%s", diff)
			}
			checkParsePrint(t, tree, root)
		})
	}
}

func TestParseExpressions(t *testing.T) {
	t.Parallel()
	type testcase struct {
		src  string
		want string
	}
	for _, tc := range []testcase{
		{src: "(int, (x > 0 && x < 10) || x == -1)", want: "(int,(x>0&&x<10)||x==-1)"},
		{src: "(int, x in #{1})", want: "(int,x in #{1})"},
		{src: "(int, !(x > 0), +x, -x, -(x + 1))", want: "(int,!(x>0),+x,-x,-(x+1))"},
		{src: "(int, x > - 1, x > -/*bound*/1.50e+2)", want: "(int,x>-1,x>-1.50e+2)"},
		{src: "(int, --1, +1, x - -1)", want: "(int,-(-1),+1,x--1)"},
		{src: "(int, fn(), pkg.fn(x, 1 + 2,))", want: "(int,fn(),pkg.fn(x,1+2))"},
		{src: "(int, .fn(.value, pkg.true,))", want: "(int,.fn(.value,pkg.true))"},
		{src: "(int, (a == b) == c, a == (b in [1]))", want: "(int,(a==b)==c,a==(b in [1]))"},
		{src: `(int, true, false, null, 1, 1.5, "value", [1], {"a":1})`, want: `(int,true,false,null,1,1.5,"value",[1],{"a":1})`},
		{src: "(int, fn([(string)|(int)], {\"value\":(int, x>0)}))", want: "(int,fn([(string)|(int)],{\"value\":(int,x>0)}))"},
		{src: "(string, len(x) > 0) | (int, x > 0)", want: "(string,len(x)>0)|(int,x>0)"},
		{src: "(int | string, x > 0)", want: "(int|string,x>0)"},
		{src: `(myObjType, len(x.tags) > 0, x.name != "")`, want: `(myObjType,len(x.tags)>0,x.name!="")`},
		{src: "(myObjType, x.valid)", want: "(myObjType,(x.valid))"},
		{src: "(myObjType, (x.valid))", want: "(myObjType,(x.valid))"},
	} {
		t.Run(tc.src, func(t *testing.T) {
			tree, root := parseOK(t, tc.src)
			if diff := cmp.Diff(tc.want, printString(t, tree, root)); diff != "" {
				t.Fatalf("printed expression differs (-want +got):\n%s", diff)
			}
			checkParsePrint(t, tree, root)
		})
	}
}

func TestParseExpressionErrors(t *testing.T) {
	t.Parallel()
	type testcase struct {
		src string
	}
	for _, tc := range []testcase{
		{src: "(int, x >)"}, {src: "(int, && x)"}, {src: "(int, x y)"},
		{src: "(int, x = y)"}, {src: "(int, x += 1)"}, {src: "(int, ! )"},
		{src: "(int, ())"}, {src: "(int, (x, y))"}, {src: "(int, (x)"},
		{src: "(int, fn(,))"}, {src: "(int, fn(x,,))"}, {src: "(int, fn(x y))"},
		{src: "(int, fn(x) y)"}, {src: "(int, fn()())"}, {src: "(int, x.)"},
		{src: "(int, .)"}, {src: "(int, x | y)"}, {src: "(int, 1true)"},
		{src: "(int, x intrue)"}, {src: "(int, x in[1]x)"}, {src: "(int, in)"},
		{src: "(int, label=1+2)"}, {src: "(int, label=(1))"}, {src: "(int, label=- 1)"},
		{src: "(int, x == y == z)"}, {src: "(int, x != y in [1])"},
		{src: `(string, x ~ "a" ~ "b")`}, {src: "(int, x == y < z != q)"},
	} {
		t.Run(tc.src, func(t *testing.T) { parseFailure(t, tc.src) })
	}
}

func TestParseConstraintCompileErrors(t *testing.T) {
	t.Parallel()
	type testcase struct {
		src  string
		span string
	}
	for _, tc := range []testcase{
		{src: "(string | int, x > 0)", span: "x > 0"},
		{src: "(int | int, x > 0)", span: "x > 0"},
		{src: "((string) | (int), x > 0)", span: "x > 0"},
		{src: "(int, x + 1 > 0)", span: "x + 1"},
		{src: "(int, x != 1)", span: "x != 1"},
		{src: "(string, x ~ \"a\")", span: "x ~ \"a\""},
		{src: "(int, fn())", span: "fn()"},
		{src: "(int, .missing > 0)", span: ".missing"},
	} {
		t.Run(tc.src, func(t *testing.T) {
			tree, root := parseOK(t, tc.src)
			_, err := compiled.Compile(compiled.NewArena(), tree, root)
			var diagnostic *compiled.CompileError
			if !errors.As(err, &diagnostic) {
				t.Fatalf("compile error = %v, want CompileError", err)
			}
			checkSpanText(t, tc.src, diagnostic.Span, tc.span)
		})
	}
}
