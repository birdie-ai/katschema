package compiled

import (
	"testing"

	"github.com/birdie-ai/katschema/ks"
	"github.com/birdie-ai/katschema/parser/ast"
	"github.com/birdie-ai/katschema/parser/token"
)

func TestRealLiteralInterningIsExact(t *testing.T) {
	t.Parallel()

	var s token.Span
	tree := ast.New()
	a := NewArena()

	// NOTE(i4k): we are interning literal decimal "1.0100" to (real, x == "1.0100")
	x := tree.AddDecimal("1.0100", s)
	y := tree.AddDecimal("1.01e0", s)
	z := tree.AddSchema(tree.AddName("real", s), []ast.NodeID{
		tree.AddConstraint(tree.AddBinary(tree.AddIdent("x", s), token.Eq, tree.AddDecimal("1.0100", s), s), s),
	}, s)

	xid, err := Compile(a, tree, x)
	if err != nil {
		t.Fatal(err)
	}
	yid, err := Compile(a, tree, y)
	if err != nil {
		t.Fatal(err)
	}
	zid, err := Compile(a, tree, z)
	if err != nil {
		t.Fatal(err)
	}
	if xid != yid {
		t.Fatalf("exactly equal real literals have different IDs: %d != %d", xid, yid)
	}
	if zid != xid {
		t.Fatalf("exactly equal real literals have different IDs: %d != %d", xid, zid)
	}
	if got := a.Node(xid).Kind(); got != Refined {
		t.Fatalf("literal kind = %s, want %s", got, Refined)
	}
}

func TestFloatAliasesAndSubtyping(t *testing.T) {
	t.Parallel()

	a := NewArena()
	f32Type := compile(t, a, ks.Float32())
	f64Type := compile(t, a, ks.Float64())
	floatAlias := compile(t, a, ks.Float())
	realType := compile(t, a, ks.Real())

	if floatAlias != f64Type {
		t.Fatalf("float alias = %d, want float64 %d", floatAlias, f64Type)
	}
	if !a.Subtype(f32Type, f64Type) {
		t.Fatal("float32 should be a subtype of float64")
	}
	if !a.Subtype(f64Type, realType) {
		t.Fatal("float64 should be a subtype of real")
	}
	if a.Subtype(f64Type, f32Type) {
		t.Fatal("float64 should not be a subtype of float32")
	}
	if got := a.internSum([2]TypeID{f32Type, f64Type}); got != f64Type {
		t.Fatalf("float32 | float64 = %d, want float64 %d", got, f64Type)
	}
}
