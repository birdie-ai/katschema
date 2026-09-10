package katschema_test

import (
	"errors"
	"testing"

	"github.com/birdie-ai/katschema"
	"github.com/birdie-ai/katschema/ks"
	. "github.com/birdie-ai/katschema/ks"
)

// NOTE(i4k): just smoke tests checking if public API "works".

func TestCompile(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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

func TestTraverse(t *testing.T) {
	t.Parallel()

	typ, err := katschema.Compile(ks.List(ks.Object(
		ks.Field("nested", ks.With(
			ks.Object(ks.Field("value", ks.String())),
			ks.Attr("some.app.marker", ks.BoolExpr(true)),
		)),
	)))
	if err != nil {
		t.Fatal(err)
	}

	got, ok := typ.Traverse(katschema.Path{"nested", "value"})
	if !ok {
		t.Fatal("Traverse returned no type")
	}
	if got.Kind() != katschema.String {
		t.Fatalf("resolved kind = %v, want %v", got.Kind(), katschema.String)
	}
	if _, ok := typ.Traverse(katschema.Path{"nested", "missing"}); ok {
		t.Fatal("Traverse found a missing field")
	}

	literal, err := katschema.Compile(ks.List(ks.LitString("a"), ks.LitString("b")))
	if err != nil {
		t.Fatal(err)
	}
	if literal.Kind() != katschema.Tuple {
		t.Fatalf("literal list kind = %v, want tuple", literal.Kind())
	}
	if got, ok := literal.Traverse(nil); !ok || got.Kind() != katschema.Tuple {
		t.Fatalf("empty path changed literal list type: %v, found=%v", got.Kind(), ok)
	}
	if _, ok := literal.Traverse(katschema.Path{"value"}); ok {
		t.Fatal("Traverse traversed a literal list as a schema list")
	}

	object, err := katschema.Compile(ks.Object(
		ks.Field("a", ks.LitInt(1)),
		ks.Field("b", ks.Object(ks.Field("c", ks.LitInt(1)))),
	))
	if err != nil {
		t.Fatal(err)
	}
	got, ok = object.Traverse(katschema.Path{"b", "c"})
	if !ok {
		t.Fatal("Traverse returned no literal field")
	}
	one, err := katschema.Compile(ks.LitInt(1))
	if err != nil {
		t.Fatal(err)
	}
	if got.SemanticFingerprint() != one.SemanticFingerprint() {
		t.Fatalf("literal field fingerprint = %d, want %d", got.SemanticFingerprint(), one.SemanticFingerprint())
	}
}

func TestSubtype(t *testing.T) {
	t.Parallel()

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

func TestOverlay(t *testing.T) {
	t.Parallel()

	cc := katschema.NewCompiler()
	base, err := cc.Compile(Object(
		Field("id", With(String(), Flag("pk"))),
		Field("text", Optional(String())),
	))
	if err != nil {
		t.Fatal(err)
	}
	top, err := cc.Compile(Object(
		Field("custom_fields", Object(Field("priority", Int()))),
		Field("text", String()),
	))
	if err != nil {
		t.Fatal(err)
	}
	typ, err := base.Overlay(top)
	if err != nil {
		t.Fatal(err)
	}

	fields := typ.Fields()
	if len(fields) != 3 || fields[0].Name != "custom_fields" || fields[1].Name != "id" || fields[2].Name != "text" {
		t.Fatalf("fields=%v, want canonical overlay fields", fields)
	}
	text, ok := typ.Field("text")
	if !ok || text.Optional {
		t.Fatalf("text=%+v, found=%v; top layer should replace the core field", text, ok)
	}
	id, ok := typ.Field("id")
	if !ok {
		t.Fatal("id field not found")
	}
	if got, ok := id.Metadata().Get("pk"); !ok || got != true {
		t.Fatalf("id metadata=%v, found=%v", got, ok)
	}

	valid := Object(
		Field("id", LitString("one")),
		Field("text", LitString("hello")),
		Field("custom_fields", Object(Field("priority", LitInt(1)))),
	)
	if err := typ.Validate(valid); err != nil {
		t.Fatalf("valid overlay value rejected: %v", err)
	}
	if err := typ.Validate(Object(
		Field("id", LitString("one")),
		Field("text", LitInt(1)),
		Field("custom_fields", Object(Field("priority", LitInt(1)))),
	)); err == nil {
		t.Fatal("invalid overlay value accepted")
	}

	nested, err := typ.Overlay(top)
	if err != nil || nested.SemanticFingerprint() != typ.SemanticFingerprint() {
		t.Fatalf("nested overlay = %v, want equivalent effective type", err)
	}
}

func TestMetadataIsPreserved(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
			Attr("some.app.marker", StrExpr("analyzed")),
		), Name: "analyzed", CacheKey: "global:analyzed"}, nil
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
	if got, ok := typ.Metadata().Get("some.app.marker"); !ok || got != "analyzed" {
		t.Fatalf("logical type metadata=%v, found=%v", got, ok)
	}
	if got := typ.Name(); got != "analyzed" {
		t.Fatalf("resolved type name=%q, want analyzed", got)
	}
	if _, err := compiler.Compile(Type("analyzed")); err != nil {
		t.Fatal(err)
	}
	if resolverCalls != 2 {
		t.Fatalf("resolver calls=%d, want one call per compile to obtain the scope key", resolverCalls)
	}
	optionalField, err := compiler.Compile(Object(Field("custom_fields", With(
		Type("analyzed"),
		Flag("optional"),
		Attr("mapping", StrExpr("custom_fields")),
	))))
	if err != nil {
		t.Fatalf("optional resolved field: %v", err)
	}
	field, ok := optionalField.Field("custom_fields")
	if !ok || !field.Optional {
		t.Fatalf("optional resolved field=%+v, found=%v", field, ok)
	}
	if got, ok := field.Type.Metadata().Get("some.app.marker"); !ok || got != "analyzed" {
		t.Fatalf("wrapped resolved logical type metadata=%v, found=%v", got, ok)
	}
	if got := field.Type.Name(); got != "analyzed" {
		t.Fatalf("wrapped resolved type name=%q, want analyzed", got)
	}
	if got, ok := field.Type.Metadata().Get("mapping"); !ok || got != "custom_fields" {
		t.Fatalf("wrapped resolved mapping metadata=%v, found=%v", got, ok)
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

func TestTypeNameIsOptionalAndNotSemantic(t *testing.T) {
	t.Parallel()

	// NOTE(i4k): this is not the right way of defining a resolver.
	// For testing purposes this wraps every non-builtin name with a test constraint.
	resolver := katschema.ResolverFunc(func(name string) (katschema.TypeResolution, error) {
		return katschema.TypeResolution{
			Value: With(Int(), Check(Binary(X(), Gt, IntExpr(0)))),
			Name:  name,
		}, nil
	})
	compiler := katschema.NewCompilerWithResolver(resolver)

	positiveInt, err := compiler.Compile(Type("positive_int"))
	if err != nil {
		t.Fatal(err)
	}
	if got := positiveInt.Name(); got != "positive_int" {
		t.Fatalf("resolved type name=%q, want positive_int", got)
	}
	bounded, err := compiler.Compile(With(
		Type("positive_int"),
		Check(Binary(X(), Lt, IntExpr(100))),
	))
	if err != nil {
		t.Fatal(err)
	}
	if got := bounded.Name(); got != "positive_int" {
		t.Fatalf("refined resolved type name=%q, want positive_int", got)
	}

	score, err := compiler.Compile(Type("score"))
	if err != nil {
		t.Fatal(err)
	}
	if got := score.Name(); got != "score" {
		t.Fatalf("resolved type name=%q, want score", got)
	}
	if positiveInt.SemanticFingerprint() != score.SemanticFingerprint() {
		t.Fatal("type names changed semantic identity")
	}
	if positiveInt.DefinitionFingerprint() == score.DefinitionFingerprint() {
		t.Fatal("different type names share definition identity")
	}

	unnamed, err := katschema.Compile(With(Int(), Check(Binary(X(), Gt, IntExpr(0)))))
	if err != nil {
		t.Fatal(err)
	}
	if got := unnamed.Name(); got != "" {
		t.Fatalf("unnamed refinement name=%q, want empty", got)
	}
}

func TestResolverRejectsRecursion(t *testing.T) {
	t.Parallel()

	resolver := katschema.ResolverFunc(func(name string) (katschema.TypeResolution, error) {
		return katschema.TypeResolution{Value: Type(map[string]string{"a": "b", "b": "a"}[name])}, nil
	})
	_, err := katschema.CompileWithResolver(Type("a"), resolver)
	if err == nil || !errors.Is(err, katschema.ErrResolveCycle) {
		t.Fatalf("recursive resolver error=%v", err)
	}
}

func TestUnknownTypeFromCompiler(t *testing.T) {
	t.Parallel()

	_, err := katschema.Compile(Type("missing"))
	if err == nil || !errors.Is(err, katschema.ErrUnknownType) {
		t.Fatalf("unknown type error=%v, want ErrUnknownType", err)
	}
}

func TestResolverPreservesConstrainedType(t *testing.T) {
	t.Parallel()

	resolver := katschema.ResolverFunc(func(name string) (katschema.TypeResolution, error) {
		if name != "language" {
			return katschema.TypeResolution{}, katschema.ErrUnknownType
		}
		return katschema.TypeResolution{Value: With(
			String(),
			Check(Binary(X(), In, ListExpr(LitString("en"), LitString("pt")))),
			Flag("logical"),
		)}, nil
	})
	typ, err := katschema.CompileWithResolver(Object(Field("language", Type("language"))), resolver)
	if err != nil {
		t.Fatal(err)
	}
	if err := typ.Validate(Object(Field("language", LitString("en")))); err != nil {
		t.Fatalf("valid language rejected: %v", err)
	}
	if err := typ.Validate(Object(Field("language", LitString("de")))); err == nil {
		t.Fatal("invalid language accepted")
	}
}
