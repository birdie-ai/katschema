package compiled

import "testing"

func TestSubtype(t *testing.T) {
	t.Parallel()

	type testcase struct {
		name string
		a    string
		b    string
		want bool
	}

	for _, tc := range []testcase{
		{name: "never is a subtype of any", a: "(never)", b: "(any)", want: true},
		{name: "any type is bigger than (never)", a: "(never)", b: "(int)", want: true},
		{name: "(never) has no subtype", a: "(int)", b: "(never)", want: false},
		{name: "(never) has really no subtype", a: "(any)", b: "(never)", want: false},
		{name: "any type is subtype of itself", a: "(int)", b: "(int)", want: true},
		{name: "smaller literal int is smaller than (int)", a: "1", b: "(int)", want: true},
		{name: "bigInt literal is smaller than (int)", a: google, b: "(int)", want: true},
		{name: "(int, x > 0) smaller than (int)", a: "(int, x > 0)", b: "(int)", want: true},
		{name: "(int) is not smaller than (int, x > 0)", a: "(int)", b: "(int, x > 0)", want: false},
		{name: "(int, x > 10) is smaller than (int, x >= 10)", a: "(int, x > 10)", b: "(int, x >= 10)", want: true},
		{name: "(int, x > 9) is smaller than (int, x >= 10)", a: "(int, x > 9)", b: "(int, x >= 10)", want: true},
		{
			name: "(int, -1 < x < 11) is equal than (int, 0 <= x <= 10)",
			a:    "(int, -1 < x < 11)",
			b:    "(int, 0 <= x <= 10)",
			want: true,
		},
		{
			name: "(int, -1 < x < 9) is smaller than (int, -1 < x < 10)",
			a:    "(int, -1 < x < 9)",
			b:    "(int, -1 < x < 10)",
			want: true,
		},
		{
			name: "(int, 0 <= x < 10) is smaller than (int, -1 < x < 10)",
			a:    "(int, 0 <= x < 10)",
			b:    "(int, -1 < x < 10)",
			want: true,
		},
		{
			name: "(int, 0 < x < 10) is smaller than (int, 0 < x < 10)",
			a:    "(int, 0 < x < 10)",
			b:    "(int, 0 < x < 10)",
			want: true,
		},
		{
			name: "narrow object is a subtype",
			a:    `{"a": (int, x > 0)}`,
			b:    `{"a": (int)}`,
			want: true,
		},
		{
			name: "object with less fields is a subtype",
			a:    `{"a": (int)}`,
			b:    `{"a": (int), "b": (int, optional)}`,
			want: true,
		},
		{
			name: "narrow list is a subtype",
			a:    "[(int, x > 0)]",
			b:    "[(int)]",
			want: true,
		},
		{
			name: "(real) subtyping honor constraint",
			a:    "(real, x > 1.0)",
			b:    "(real, x >= 1.0)",
			want: true,
		},
		{
			name: "(real) subtyping honor constraint - reverse",
			b:    "(real, x > 1.0)",
			a:    "(real, x >= 1.0)",
		},
		{name: "check (int) is a subtype of (real)", a: "(int)", b: "(real)", want: true},
		{name: "check (float32) is a subtype of (float64)", a: "(float32)", b: "(float64)", want: true},
		{name: "check (float64) is not a subtype of (float32)", a: "(float64)", b: "(float32)", want: false},
		{name: "check (float32) is a subtype of (real)", a: "(float32)", b: "(real)", want: true},
		{name: "check (float64) is a subtype of (real)", a: "(float64)", b: "(real)", want: true},
		{name: "check (real) is not a subtype of (float32)", a: "(real)", b: "(float32)", want: false},
		{name: "check (real) is not a subtype of (float64)", a: "(real)", b: "(float64)", want: false},
		{name: "check (float) is an alias of (float64)", a: "(float)", b: "(float64)", want: true},
		{name: "check (real) is not a subtype of (int)", b: "(int)", a: "(real)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			arena := NewArena()
			a := compileText(t, arena, tc.a)
			b := compileText(t, arena, tc.b)
			if got := arena.Subtype(a, b); got != tc.want {
				t.Fatalf("Subtype(a, b) = %v, want %v", got, tc.want)
			}
			other := NewArena()
			compileText(t, other, `{"padding": [(string)]}`)
			otherB := compileText(t, other, tc.b)
			if got := arena.Type(a).SubtypeOf(other.Type(otherB)); got != tc.want {
				t.Fatalf("cross-arena Subtype(a, b) = %v, want %v", got, tc.want)
			}
			if a != b {
				if got := arena.Subtype(b, a); got != !tc.want {
					t.Fatalf("reversed Subtype(b, a) = %v, want %v", got, !tc.want)
				}
			}
		})
	}
}
