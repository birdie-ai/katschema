package katschema_test

import (
	"testing"

	"github.com/birdie-ai/katschema"
	. "github.com/birdie-ai/katschema/ks"
)

// NOTE(i4k): just smoke tests checking if public API "works".

func TestCompile(t *testing.T) {
	typ, err := katschema.Compile(Object(
		Field("id", String()),
		Field("tags", Optional(List(String()))),
	))
	if err != nil {
		t.Fatal(err)
	}
	if typ.Kind() != katschema.Object {
		t.Fatalf("kind=%v, want object", typ.Kind())
	}
	if len(typ.Fields()) != 2 {
		t.Fatalf("fields=%d, want 2", len(typ.Fields()))
	}
	field, ok := typ.Field("tags")
	if !ok || !field.Optional || field.Type.Kind() != katschema.List {
		t.Fatalf("unexpected tags field: %+v, found=%v", field, ok)
	}
	if _, ok := field.Type.Element(); !ok {
		t.Fatal("tags should have an element type")
	}
}

func TestValidate(t *testing.T) {
	typ, err := katschema.Compile(Object(
		Field("id", String()),
		Field("age", Optional(Int())),
	))
	if err != nil {
		t.Fatal(err)
	}

	if err := typ.Validate(Object(Field("id", LitString("one")))); err != nil {
		t.Fatalf("valid value rejected: %v", err)
	}
	if err := typ.Validate(Object(Field("id", LitInt(1)))); err == nil {
		t.Fatal("invalid value accepted")
	}
}

func TestSubtype(t *testing.T) {
	compiler := katschema.NewCompiler()
	required, err := compiler.Compile(Object(Field("id", String())))
	if err != nil {
		t.Fatal(err)
	}
	optional, err := compiler.Compile(Object(Field("id", Optional(String()))))
	if err != nil {
		t.Fatal(err)
	}
	if !required.SubtypeOf(optional) {
		t.Fatal("required object should be a subtype of optional object")
	}
}

func TestMetadataIsPreserved(t *testing.T) {
	cc := katschema.NewCompiler()
	withOptions, err := cc.Compile(Object(
		Field("id", With(String(), Flag("pk"), Attr("owner", StrExpr("ingestion")))),
	))
	if err != nil {
		t.Fatal(err)
	}
	withoutOptions, err := cc.Compile(Object(Field("id", String())))
	if err != nil {
		t.Fatal(err)
	}

	field, ok := withOptions.Field("id")
	if !ok {
		t.Fatal("id field not found")
	}
	if got, ok := field.Metadata().Get("pk"); !ok || got != true {
		t.Fatalf("pk metadata=%v, found=%v", got, ok)
	}
	if got, ok := field.Metadata().Get("owner"); !ok || got != "ingestion" {
		t.Fatalf("owner metadata=%v, found=%v", got, ok)
	}
	if withOptions.SemanticFingerprint() != withoutOptions.SemanticFingerprint() {
		withField, _ := withOptions.Field("id")
		withoutField, _ := withoutOptions.Field("id")
		t.Fatalf("metadata should not change semantic fingerprint: got %d and %d, kinds=%v/%v", withOptions.SemanticFingerprint(), withoutOptions.SemanticFingerprint(), withField.Type.Kind(), withoutField.Type.Kind())
	}
	if withOptions.DefinitionFingerprint() == withoutOptions.DefinitionFingerprint() {
		t.Fatal("metadata should change definition fingerprint")
	}
}
