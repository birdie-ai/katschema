package parser

import (
	"testing"

	"github.com/birdie-ai/katschema/parser/ast"
	"github.com/google/go-cmp/cmp"
)

func TestParseTuple(t *testing.T) {
	t.Parallel()
	const tuple = "#{/* α */(string, len(x) > 0), #{1,}, // last\n [true],}"
	const src = " " + tuple + " "
	tree, root := parseOK(t, src)
	if got := tree.Node(root).Kind(); got != ast.Tuple {
		t.Fatalf("root kind = %v, want Tuple", got)
	}
	checkSpanText(t, src, tree.Node(root).Span(), tuple)
	elems := tree.Tuple(root)
	var kinds []ast.Kind
	for _, elem := range elems {
		kinds = append(kinds, tree.Node(elem).Kind())
	}
	if diff := cmp.Diff([]ast.Kind{ast.Schema, ast.Tuple, ast.List}, kinds); diff != "" {
		t.Fatalf("tuple elements differ (-want +got):\n%s", diff)
	}
	checkSpanText(t, src, tree.Node(elems[0]).Span(), "(string, len(x) > 0)")
	checkSpanText(t, src, tree.Node(elems[1]).Span(), "#{1,}")
	checkSpanText(t, src, tree.Node(elems[2]).Span(), "[true]")
	var comments []string
	for _, c := range tree.Comments() {
		comment := tree.Text(c.Text)
		checkSpanText(t, src, c.Span, comment)
		comments = append(comments, comment)
	}
	if diff := cmp.Diff([]string{"/* α */", "// last"}, comments); diff != "" {
		t.Fatalf("tuple comments differ (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff("#{(string,len(x)>0),#{1},[true]}", printString(t, tree, root)); diff != "" {
		t.Fatalf("printed tuple differs (-want +got):\n%s", diff)
	}
	checkParsePrint(t, tree, root)
}
