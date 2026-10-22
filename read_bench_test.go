package katschema_test

import (
	"fmt"
	"testing"

	"github.com/birdie-ai/katschema"
	"github.com/birdie-ai/katschema/ks"
)

func BenchmarkTraverseViews(b *testing.B) {
	core := compile(b, katschema.NewCompiler(), ks.List(ks.With(ks.Object(
		ks.Field("id", ks.String()),
		ks.Field("custom_fields", ks.Optional(ks.Object())),
	), ks.Attr("owner", ks.StrExpr("core")))))
	patch := compile(b, katschema.NewCompiler(), ks.Object(
		ks.Field("custom_fields", ks.Optional(ks.Object(
			ks.Field("priority", ks.With(ks.Int(), ks.Check(ks.Binary(ks.X(), ks.Gt, ks.IntExpr(0))))),
			ks.Field("tags", ks.List(ks.Object(ks.Field("name", ks.String())))),
		))),
	))
	overlay, err := katschema.Overlay(core, patch)
	if err != nil {
		b.Fatal(err)
	}
	type testcase struct {
		name  string
		typ   katschema.Type
		path  katschema.Path
		found bool
	}
	for _, tc := range []testcase{
		{
			name:  "Core",
			typ:   core,
			path:  katschema.Path{"id"},
			found: true,
		},
		{
			name:  "OverlayCore",
			typ:   overlay,
			path:  katschema.Path{"id"},
			found: true,
		},
		{
			name:  "OverlayDynamic",
			typ:   overlay,
			path:  katschema.Path{"custom_fields", "priority"},
			found: true,
		},
		{
			name:  "OverlayNested",
			typ:   overlay,
			path:  katschema.Path{"custom_fields", "tags", "name"},
			found: true,
		},
		{
			name: "OverlayMissing",
			typ:  overlay,
			path: katschema.Path{"custom_fields", "missing"},
		},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, ok := tc.typ.Traverse(tc.path); ok != tc.found {
					b.Fatal("unexpected traversal result")
				}
			}
		})
	}
}

func BenchmarkTraverseFieldCounts(b *testing.B) {
	type testcase struct {
		name string
		typ  katschema.Type
		kind katschema.Kind
	}

	for _, count := range []int{4, 32, 128} {
		fields := make([]ks.Member, count)
		for i := range fields {
			fields[i] = ks.Field(fmt.Sprintf("f%03d", i), ks.Int())
		}
		core := compile(b, katschema.NewCompiler(), ks.Object(fields...))
		path := katschema.Path{fmt.Sprintf("f%03d", count/2)}
		patch := compile(b, katschema.NewCompiler(), ks.Object(ks.Field(path[0], ks.String())))
		overlay, err := katschema.Overlay(core, patch)
		if err != nil {
			b.Fatal(err)
		}
		for _, tc := range []testcase{
			{
				name: "Core",
				typ:  core,
				kind: katschema.Int,
			},
			{
				name: "Overlay",
				typ:  overlay,
				kind: katschema.String,
			},
		} {
			b.Run(fmt.Sprintf("%s/%d", tc.name, count), func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					got, ok := tc.typ.Traverse(path)
					if !ok || got.Kind() != tc.kind {
						b.Fatal("unexpected traversal result")
					}
				}
			})
		}
	}
}
