package katschema_test

import (
	"errors"
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

func TestResolver(t *testing.T) {
	resolverCalls := 0
	resolver := katschema.ResolverFunc(func(name string) (katschema.TypeResolution, error) {
		resolverCalls++
		if name != "analyzed" {
			return katschema.TypeResolution{}, katschema.ErrUnknownType
		}
		return katschema.TypeResolution{Value: With(
			Sum(
				String(),
				Object(
					Field("en", Optional(String())),
					Field("und", Optional(String())),
				),
			),
			Attr("search.logical_type", StrExpr("analyzed")),
		), CacheKey: "global:analyzed"}, nil
	})
	compiler := katschema.NewCompilerWithResolver(resolver)
	typ, err := compiler.Compile(Type("analyzed"))
	if err != nil {
		t.Fatal(err)
	}
	base, ok := typ.Base()
	if typ.Kind() != katschema.Refined || !ok || base.Kind() != katschema.Sum || len(base.Variants()) != 2 {
		t.Fatalf("resolved analyzed type=%v, base=%v, variants=%d", typ.Kind(), base.Kind(), len(base.Variants()))
	}
	if got, ok := typ.Metadata().Get("search.logical_type"); !ok || got != "analyzed" {
		t.Fatalf("logical type metadata=%v, found=%v", got, ok)
	}
	if _, err := compiler.Compile(Type("analyzed")); err != nil {
		t.Fatal(err)
	}
	if resolverCalls != 2 {
		t.Fatalf("resolver calls=%d, want one call per compile to obtain the scope key", resolverCalls)
	}
	optionalField, err := compiler.Compile(Object(Field("custom_fields", Optional(Type("analyzed")))))
	if err != nil {
		t.Fatalf("optional resolved field: %v", err)
	}
	field, ok := optionalField.Field("custom_fields")
	if !ok || !field.Optional {
		t.Fatalf("optional resolved field=%+v, found=%v", field, ok)
	}

	customer := "abc"
	customerCompiler := katschema.NewCompilerWithResolver(katschema.ResolverFunc(func(name string) (katschema.TypeResolution, error) {
		return katschema.TypeResolution{
			Value:    With(String(), Attr("customer", StrExpr(customer))),
			CacheKey: "custom_fields:" + customer,
		}, nil
	}))
	abc, err := customerCompiler.Compile(Type("custom_fields"))
	if err != nil {
		t.Fatal(err)
	}
	customer = "xyz"
	xyz, err := customerCompiler.Compile(Type("custom_fields"))
	if err != nil {
		t.Fatal(err)
	}
	if abc.DefinitionFingerprint() == xyz.DefinitionFingerprint() {
		t.Fatal("customer-scoped cache keys must not reuse another customer's definition")
	}

	_, err = katschema.CompileWithResolver(Type("missing"), resolver)
	if err == nil || !errors.Is(err, katschema.ErrUnknownType) {
		t.Fatalf("missing type error=%v, want ErrUnknownType", err)
	}
}

func TestResolverRejectsRecursion(t *testing.T) {
	resolver := katschema.ResolverFunc(func(name string) (katschema.TypeResolution, error) {
		return katschema.TypeResolution{Value: Type(map[string]string{"a": "b", "b": "a"}[name])}, nil
	})
	_, err := katschema.CompileWithResolver(Type("a"), resolver)
	if err == nil || !errors.Is(err, katschema.ErrResolveCycle) {
		t.Fatalf("recursive resolver error=%v", err)
	}
}

func TestUnknownTypeFromCompiler(t *testing.T) {
	_, err := katschema.Compile(Type("missing"))
	if err == nil || !errors.Is(err, katschema.ErrUnknownType) {
		t.Fatalf("unknown type error=%v, want ErrUnknownType", err)
	}
}
