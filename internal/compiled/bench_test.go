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
	))
	if err != nil {
		b.Fatal(err)
	}
	typ, err := Compile(a, schemaTree, schemaRoot)
	if err != nil {
		b.Fatal(err)
	}
	valueTree, valueRoot, err := ks.Build(ks.Object(
		ks.Field("id", ks.LitInt("137")),
		ks.Field("name", ks.LitString("Richard")),
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
