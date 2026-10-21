package compiled

import (
	"math/big"
	"testing"

	"github.com/birdie-ai/katschema/ks"
)

func TestCrossArenaRelationsDoNotTrustIDsOrHashes(t *testing.T) {
	t.Parallel()
	huge, _ := new(big.Int).SetString("1234567890123456789012345678901234567890", 10)
	for _, tc := range []struct {
		name string
		a    ks.Value
		b    ks.Value
		want bool
	}{
		{
			name: "distinct literals",
			a:    ks.LitInt(7),
			b:    ks.LitInt(8),
		},
		{
			name: "big integer",
			a:    ks.LitBigInt(huge),
			b:    ks.With(ks.Int(), ks.Check(ks.Binary(ks.X(), ks.Gt, ks.IntExpr(0)))),
			want: true,
		},
		{
			name: "big integer equality",
			a:    ks.LitBigInt(huge),
			b:    ks.LitBigInt(huge),
			want: true,
		},
		{
			name: "enum membership",
			a:    ks.LitString("ready"),
			b:    ks.With(ks.String(), ks.Check(ks.Binary(ks.X(), ks.In, ks.ListExpr(ks.LitString("ready"), ks.LitString("done"))))),
			want: true,
		},
		{
			name: "enum rejection",
			a:    ks.LitString("other"),
			b:    ks.With(ks.String(), ks.Check(ks.Binary(ks.X(), ks.In, ks.ListExpr(ks.LitString("ready"), ks.LitString("done"))))),
		},
		{
			name: "real boundary",
			a:    ks.LitDecimal("1.1"),
			b:    ks.With(ks.Real(), ks.Check(ks.Binary(ks.X(), ks.Gt, ks.DecimalExpr("1.10")))),
		},
		{
			name: "sum source",
			a:    ks.Sum(ks.LitString("a"), ks.LitInt(1)),
			b:    ks.Sum(ks.String(), ks.Int()),
			want: true,
		},
		{
			name: "distinct object fields",
			a:    ks.Object(ks.Field("a", ks.String())),
			b:    ks.Object(ks.Field("b", ks.String())),
		},
		{
			name: "Unicode string length",
			a:    ks.LitString("é"),
			b:    ks.With(ks.String(), ks.Check(ks.Binary(ks.Funcall("len", ks.X()), ks.Le, ks.IntExpr(1)))),
			want: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Both arenas intentionally give every definition the same hash.
			a, b := newArena(func([]byte) uint64 { return 1 }), newArena(func([]byte) uint64 { return 1 })
			at, bt := a.Type(compile(t, a, tc.a)), b.Type(compile(t, b, tc.b))
			if at.Fingerprint() != bt.Fingerprint() {
				t.Fatal("test must force matching hashes")
			}
			if got := at.SubtypeOf(bt); got != tc.want {
				t.Fatalf("SubtypeOf = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSumRetainsSemanticallyEquivalentMembers(t *testing.T) {
	t.Parallel()
	a := NewArena()
	got := a.Type(compile(t, a, ks.Sum(
		ks.With(ks.String(), ks.Attr("owner", ks.StrExpr("a"))),
		ks.With(ks.String(), ks.Attr("owner", ks.StrExpr("b"))),
		ks.Int(),
	)))
	want := a.Type(compile(t, a, ks.Sum(ks.String(), ks.Int())))
	if !got.Equivalent(want) {
		t.Fatal("sum dropped all representatives of equivalent definitions")
	}
}
