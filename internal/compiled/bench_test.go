package compiled

import (
	"testing"

	"github.com/birdie-ai/katschema/ks"
)

func BenchmarkCompileInternObject(b *testing.B) {
	tree, root, err := ks.Build(ks.Object(
		ks.Field("id", ks.Int()),
		ks.Field("name", ks.String()),
		ks.Field("email", ks.Optional(ks.String())),
	))
	if err != nil {
		b.Fatal(err)
	}
	a := NewArena()
	if _, err := Compile(a, tree, root); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := Compile(a, tree, root); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidateObject(b *testing.B) {
	a := NewArena()
	schemaTree, schemaRoot, err := ks.Build(ks.Object(
		ks.Field("id", ks.Int()),
		ks.Field("name", ks.String()),
		ks.Field("email", ks.Optional(ks.String())),
		ks.Field("age", ks.With(ks.Int(), ks.Check(
			ks.Binary(
				ks.X(),
				ks.Ge,
				ks.IntExpr(18),
			),
		))),
	))
	if err != nil {
		b.Fatal(err)
	}
	typ, err := Compile(a, schemaTree, schemaRoot)
	if err != nil {
		b.Fatal(err)
	}
	valueTree, valueRoot, err := ks.Build(ks.Object(
		ks.Field("id", ks.LitInt(67)),
		ks.Field("name", ks.LitString("Richard Feymann")),
		ks.Field("age", ks.LitInt(69)),
	))
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if !a.Valid(typ, valueTree, valueRoot) {
			b.Fatal("unexpected validation failure")
		}
	}
}

func BenchmarkValidateTuple(b *testing.B) {
	a := NewArena()
	typ := compileText(b, a, "#{(string, len(x) > 0), (int, x >= 0)}")
	type testcase struct {
		name string
		src  string
	}
	for _, tc := range []testcase{
		{name: "array", src: `["row",42]`},
		{name: "tuple", src: `#{"row",42}`},
	} {
		b.Run(tc.name, func(b *testing.B) {
			tree, root := parseText(b, tc.src)
			b.ReportAllocs()
			for b.Loop() {
				if !a.Valid(typ, tree, root) {
					b.Fatal("invalid tuple fixture")
				}
			}
		})
	}
}
