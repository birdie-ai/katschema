package compiled

import (
	"testing"

	"github.com/birdie-ai/katschema/parser"
	"github.com/birdie-ai/katschema/parser/ast"
)

func parseText(t testing.TB, src string) (*ast.Tree, ast.NodeID) {
	t.Helper()
	tree, root, err := parser.Parse(src)
	if err != nil {
		t.Fatalf("parse %q: %v", src, err)
	}
	return tree, root
}

func compileText(t testing.TB, a *Arena, src string) TypeID {
	t.Helper()
	tree, root := parseText(t, src)
	id, err := Compile(a, tree, root)
	if err != nil {
		t.Fatalf("compile %q: %v", src, err)
	}
	return id
}
