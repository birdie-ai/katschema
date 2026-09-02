package compiled

import (
	"testing"

	"github.com/birdie-ai/katschema/ks"
	"github.com/birdie-ai/katschema/parser/ast"
	"github.com/birdie-ai/katschema/parser/token"
)

func TestRealLiteralInterningIsExact(t *testing.T) {
	t.Parallel()

	var z token.Span
	tree := ast.New()
	a := NewArena()
	x := tree.AddDecimal("1.0100", z)
	y := tree.AddDecimal("1.01e0", z)

	xid, err := Compile(a, tree, x)
	if err != nil {
		t.Fatal(err)
	}
	yid, err := Compile(a, tree, y)
	if err != nil {
		t.Fatal(err)
	}
	if xid != yid {
		t.Fatalf("exactly equal real literals have different IDs: %d != %d", xid, yid)
	}
	if got := a.Node(xid).Kind(); got != RealLit {
		t.Fatalf("literal kind = %s, want %s", got, RealLit)
	}
}

func TestFloatConstraintsStillUseBinary64Literals(t *testing.T) {
	t.Parallel()

	a := NewArena()
	typ := compile(t, a, ks.Where(ks.Float(), ks.Binary(ks.X(), ks.Eq, ks.FloatExpr(0.1))))
	tree, value := build(t, ks.LitReal("0.10000000000000001"))
	if !a.Valid(typ, tree, value) {
		t.Fatal("legacy float equality should use the binary64 value")
	}
}
