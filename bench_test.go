package katschema_test

import (
	"context"
	"testing"

	"github.com/birdie-ai/katschema"
	"github.com/birdie-ai/katschema/ks"
)

func BenchmarkTraverse(b *testing.B) {
	typ, err := katschema.Compile(context.Background(), ks.List(ks.With(
		ks.Object(
			ks.Field("id", ks.String()),
			ks.Field("child", ks.With(
				ks.List(ks.With(
					ks.Object(
						ks.Field("kind", ks.String()),
						ks.Field("account", ks.Object(
							ks.Field("id", ks.String()),
							ks.Field("name", ks.String()),
						)),
					),
					ks.Attr("some.attr", ks.BoolExpr(true)),
				)),
				ks.Attr("some.marker", ks.BoolExpr(true)),
			)),
		),
		ks.Attr("some.entity", ks.StrExpr("feedbacks")),
	)))
	if err != nil {
		b.Fatal(err)
	}
	path := katschema.Path{"child", "account", "name"}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		got, ok := typ.Traverse(path)
		if !ok || got.Kind() != katschema.String {
			b.Fatal("unexpected traversal result")
		}
	}
}

func BenchmarkCrossArenaOverlay(b *testing.B) {
	core := compile(b, katschema.NewCompiler(), ks.List(ks.With(
		ks.Object(
			ks.Field("id", ks.String()),
			ks.Field("custom_fields", ks.Optional(ks.Object())),
		),
		ks.Attr("pks", ks.ListExpr(ks.LitString("id"))),
	)))
	topValue := ks.Object(ks.Field("custom_fields", ks.Optional(ks.Object(
		ks.Field("priority", ks.With(ks.Real(), ks.Attr("mapping", ks.StrExpr("numbers")))),
	))))
	top := compile(b, katschema.NewCompiler(), topValue)
	typ, err := katschema.Overlay(core, top)
	if err != nil {
		b.Fatal(err)
	}
	otherTop := compile(b, katschema.NewCompiler(), topValue)
	other, err := katschema.Overlay(core, otherTop)
	if err != nil {
		b.Fatal(err)
	}
	priority, _ := typ.Traverse(katschema.Path{"custom_fields", "priority"})
	literal := compile(b, katschema.NewCompiler(), ks.LitInt(7))
	b.Run("Overlay", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if _, err := katschema.Overlay(core, top); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("Subtype", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if !literal.SubtypeOf(priority) {
				b.Fatal("unexpected subtype")
			}
		}
	})
	b.Run("Equivalent", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if !typ.Equivalent(other) {
				b.Fatal("unexpected equivalence")
			}
		}
	})
	b.Run("Fingerprint", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if typ.Fingerprint() != other.Fingerprint() || typ.SemanticFingerprint() != other.SemanticFingerprint() {
				b.Fatal("unexpected fingerprints")
			}
		}
	})
}
