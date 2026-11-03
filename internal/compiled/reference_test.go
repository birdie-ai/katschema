package compiled

import (
	"testing"

	"github.com/birdie-ai/katschema/ks"
)

func TestCrossArenaRelationsDoNotTrustIDsOrHashes(t *testing.T) {
	t.Parallel()
	const huge = "1234567890123456789012345678901234567890"
	type testcase struct {
		name string
		a    string
		b    string
		want bool
	}
	for _, tc := range []testcase{
		{
			name: "distinct literals",
			a:    "7",
			b:    "8",
		},
		{
			name: "big integer",
			a:    huge,
			b:    "(int, x > 0)",
			want: true,
		},
		{
			name: "big integer equality",
			a:    huge,
			b:    huge,
			want: true,
		},
		{
			name: "enum membership",
			a:    `"ready"`,
			b:    `(string, x in ["ready", "done"])`,
			want: true,
		},
		{
			name: "enum rejection",
			a:    `"other"`,
			b:    `(string, x in ["ready", "done"])`,
		},
		{
			name: "real boundary",
			a:    "1.1",
			b:    "(real, x > 1.10)",
		},
		{
			name: "distinct object fields",
			a:    `{"a": (string)}`,
			b:    `{"b": (string)}`,
		},
		{
			name: "Unicode string length",
			a:    `"é"`,
			b:    "(string, len(x) <= 1)",
			want: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Both arenas intentionally give every definition the same hash.
			a, b := newArena(func([]byte) uint64 { return 1 }), newArena(func([]byte) uint64 { return 1 })
			at, bt := a.Type(compileText(t, a, tc.a)), b.Type(compileText(t, b, tc.b))
			if at.Fingerprint() != bt.Fingerprint() {
				t.Fatal("test must force matching hashes")
			}
			if got := at.SubtypeOf(bt); got != tc.want {
				t.Fatalf("SubtypeOf = %v, want %v", got, tc.want)
			}
		})
	}
	t.Run("sum source", func(t *testing.T) {
		a, b := newArena(func([]byte) uint64 { return 1 }), newArena(func([]byte) uint64 { return 1 })
		at := a.Type(compile(t, a, ks.Sum(ks.LitString("a"), ks.LitInt(1))))
		bt := b.Type(compileText(t, b, "(string) | (int)"))
		if at.Fingerprint() != bt.Fingerprint() {
			t.Fatal("test must force matching hashes")
		}
		if !at.SubtypeOf(bt) {
			t.Fatal("literal sum should be a subtype of string | int")
		}
	})
}

func TestSumRetainsSemanticallyEquivalentMembers(t *testing.T) {
	t.Parallel()
	a := NewArena()
	got := a.Type(compileText(t, a, `(string, owner="a") | (string, owner="b") | (int)`))
	want := a.Type(compileText(t, a, "(string) | (int)"))
	if !got.Equivalent(want) {
		t.Fatal("sum dropped all representatives of equivalent definitions")
	}
}
