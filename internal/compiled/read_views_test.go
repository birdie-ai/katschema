package compiled

import (
	"fmt"
	"testing"

	"github.com/birdie-ai/katschema/ks"
)

func TestFieldLookupRepresentations(t *testing.T) {
	t.Parallel()

	type testcase struct {
		name string
		typ  Type
	}

	for _, size := range []int{0, 1, 4, 32, 129} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			a := NewArena()
			fields := make([]ks.Member, size)
			for i := range fields {
				fields[i] = ks.Field(fmt.Sprintf("f%03d", i), ks.Optional(ks.Int()))
			}
			core := a.Type(compile(t, a, ks.Object(fields...)))
			b := NewArena()
			patch := b.Type(compile(t, b, ks.Object()))
			if size > 0 {
				patch = b.Type(compile(t, b, ks.Object(ks.Field(fmt.Sprintf("f%03d", size/2), ks.With(
					ks.String(), ks.Attr("owner", ks.StrExpr("request")),
				)))))
			}
			overlay, err := Overlay(core, patch)
			if err != nil {
				t.Fatal(err)
			}
			for _, tc := range []testcase{
				{
					name: "core",
					typ:  core,
				},
				{
					name: "overlay",
					typ:  overlay,
				},
			} {
				t.Run(tc.name, func(t *testing.T) {
					got := tc.typ.Fields()
					for i := 0; i < got.Len(); i++ {
						want := got.At(i)
						field, ok := got.Get(want.Name())
						if !ok || field != want {
							t.Fatalf("lookup of %s lost type, metadata, or arena", want.Name())
						}
					}
					for _, name := range []string{"", "a", "f000x", "f999", "z"} {
						if _, ok := got.Get(name); ok {
							t.Fatalf("found absent field %q", name)
						}
					}
				})
			}
		})
	}
}

func TestLargeObjectViewValidation(t *testing.T) {
	t.Parallel()

	fields := make([]ks.Member, 65)
	values := make([]ks.Member, 65)
	for i := range fields {
		name := fmt.Sprintf("f%03d", i)
		fields[i] = ks.Field(name, ks.Int())
		values[i] = ks.Field(name, ks.LitInt(int64(i)))
	}
	a, b := NewArena(), NewArena()
	core := a.Type(compile(t, a, ks.Object(fields...)))
	patch := b.Type(compile(t, b, ks.Object(ks.Field("f032", ks.With(
		ks.String(), ks.Check(ks.Binary(ks.X(), ks.In, ks.ListExpr(ks.LitString("high"))))),
	))))
	overlay, err := Overlay(core, patch)
	if err != nil {
		t.Fatal(err)
	}
	scoped := append([]ks.Member(nil), values...)
	scoped[32] = ks.Field("f032", ks.LitString("high"))
	invalid := append([]ks.Member(nil), scoped...)
	invalid[32] = ks.Field("f032", ks.LitString("unknown"))
	// Keep the field count at 65 so duplicate rejection exercises the seen bitset.
	duplicate := append(append([]ks.Member(nil), scoped[1:]...), scoped[64])

	type testcase struct {
		name  string
		typ   Type
		value ks.Value
		valid bool
	}

	for _, tc := range []testcase{
		{
			name:  "core",
			typ:   core,
			value: ks.Object(values...),
			valid: true,
		},
		{
			name:  "overlay",
			typ:   overlay,
			value: ks.Object(scoped...),
			valid: true,
		},
		{
			name:  "old value rejected",
			typ:   overlay,
			value: ks.Object(values...),
		},
		{
			name:  "foreign refinement checked",
			typ:   overlay,
			value: ks.Object(invalid...),
		},
		{
			name:  "missing required field",
			typ:   overlay,
			value: ks.Object(scoped[:64]...),
		},
		{
			name:  "duplicate field",
			typ:   overlay,
			value: ks.Object(duplicate...),
		},
		{
			name:  "extra field",
			typ:   overlay,
			value: ks.Object(append(scoped, ks.Field("extra", ks.LitInt(1)))...),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tree, root, err := ks.Build(tc.value)
			if err != nil {
				t.Fatal(err)
			}
			if got := tc.typ.Valid(tree, root); got != tc.valid {
				t.Fatalf("Valid = %v, want %v", got, tc.valid)
			}
		})
	}
}
