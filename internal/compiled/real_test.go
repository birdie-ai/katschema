package compiled

import "testing"

func TestRealLiteralInterningIsExact(t *testing.T) {
	t.Parallel()

	a := NewArena()
	xid := compileText(t, a, "1.0100")
	yid := compileText(t, a, "1.01e0")
	zid := compileText(t, a, "(real, x == 1.0100)")
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
	f32Type := compileText(t, a, "(float32)")
	f64Type := compileText(t, a, "(float64)")
	floatAlias := compileText(t, a, "(float)")
	realType := compileText(t, a, "(real)")

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
