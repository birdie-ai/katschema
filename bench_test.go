package katschema_test

import (
	"testing"

	"github.com/birdie-ai/katschema"
	"github.com/birdie-ai/katschema/ks"
)

func BenchmarkTraverse(b *testing.B) {
	typ, err := katschema.Compile(ks.List(ks.With(
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
