package compiled

import (
	"testing"

	"github.com/birdie-ai/katschema/ks"
)

func BenchmarkValidateObjectViews(b *testing.B) {
	build := func(a *Arena, value ks.Value) Type {
		tree, root, err := ks.Build(value)
		if err != nil {
			b.Fatal(err)
		}
		id, err := Compile(a, tree, root)
		if err != nil {
			b.Fatal(err)
		}
		return a.Type(id)
	}
	core := build(NewArena(), ks.With(ks.Object(
		ks.Field("id", ks.String()),
		ks.Field("custom_fields", ks.Optional(ks.Object())),
	), ks.Attr("owner", ks.StrExpr("core"))))
	patch := build(NewArena(), ks.Object(ks.Field("custom_fields", ks.Optional(ks.Object(
		ks.Field("priority", ks.Int()),
		ks.Field("tags", ks.List(ks.String())),
	)))))
	overlay, err := Overlay(core, patch)
	if err != nil {
		b.Fatal(err)
	}

	type testcase struct {
		name  string
		typ   Type
		value ks.Value
		valid bool
	}

	for _, tc := range []testcase{
		{
			name:  "Core",
			typ:   core,
			value: ks.Object(ks.Field("id", ks.LitString("one"))),
			valid: true,
		},
		{
			name: "Overlay",
			typ:  overlay,
			value: ks.Object(
				ks.Field("id", ks.LitString("one")),
				ks.Field("custom_fields", ks.Object(
					ks.Field("priority", ks.LitInt(3)),
					ks.Field("tags", ks.List(ks.LitString("high"), ks.LitString("new"))),
				)),
			),
			valid: true,
		},
		{
			name: "OverlayInvalid",
			typ:  overlay,
			value: ks.Object(
				ks.Field("id", ks.LitString("one")),
				ks.Field("custom_fields", ks.Object(
					ks.Field("priority", ks.LitString("wrong")),
					ks.Field("tags", ks.List()),
				)),
			),
		},
	} {
		b.Run(tc.name, func(b *testing.B) {
			tree, root, err := ks.Build(tc.value)
			if err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if tc.typ.Valid(tree, root) != tc.valid {
					b.Fatal("unexpected validation result")
				}
			}
		})
	}
}
