package katschema_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/birdie-ai/katschema"
	"github.com/birdie-ai/katschema/ks"
	. "github.com/birdie-ai/katschema/ks"
	"github.com/google/go-cmp/cmp"
)

// NOTE(i4k): just smoke tests checking if public API "works".

func TestCompile(t *testing.T) {
	t.Parallel()

	typ, err := katschema.Compile(t.Context(), Object(
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

	typ, err := katschema.Compile(t.Context(), Object(
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

	typ, err := katschema.Compile(t.Context(), ks.List(ks.Object(
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

	literal, err := katschema.Compile(t.Context(), ks.List(ks.LitString("a"), ks.LitString("b")))
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

	object, err := katschema.Compile(t.Context(), ks.Object(
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
	one, err := katschema.Compile(t.Context(), ks.LitInt(1))
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
	required, err := compiler.Compile(t.Context(), Object(Field("id", String())))
	if err != nil {
		t.Fatal(err)
	}
	optional, err := compiler.Compile(t.Context(), Object(Field("id", Optional(String()))))
	if err != nil {
		t.Fatal(err)
	}
	if !required.SubtypeOf(optional) {
		t.Fatal("required object should be a subtype of optional object")
	}
}

func TestCrossCompilerTypeRelations(t *testing.T) {
	t.Parallel()

	baseCompiler := katschema.NewCompiler()
	otherCompiler := katschema.NewCompiler()
	if _, err := otherCompiler.Compile(t.Context(), Object(
		Field("padding", Sum(String(), List(Int()))),
	)); err != nil {
		t.Fatal(err)
	}

	positive := With(Int(), Check(Binary(X(), Gt, IntExpr(0))), Attr("owner", StrExpr("base")))
	positiveBase, err := baseCompiler.Compile(t.Context(), positive)
	if err != nil {
		t.Fatal(err)
	}
	positiveOther, err := otherCompiler.Compile(t.Context(), With(
		Int(),
		Check(Binary(X(), Gt, IntExpr(0))),
		Attr("owner", StrExpr("other")),
	))
	if err != nil {
		t.Fatal(err)
	}
	if !positiveBase.Equivalent(positiveOther) {
		t.Fatal("same accepted values from different compilers should be equivalent")
	}
	if positiveBase.DefinitionFingerprint() == positiveOther.DefinitionFingerprint() {
		t.Fatal("different metadata should retain different definition fingerprints")
	}

	nonnegative, err := otherCompiler.Compile(t.Context(), With(
		Int(), Check(Binary(X(), Ge, IntExpr(0))),
	))
	if err != nil {
		t.Fatal(err)
	}
	if !positiveBase.SubtypeOf(nonnegative) {
		t.Fatal("positive integers should be a subtype of nonnegative integers across compilers")
	}
	if nonnegative.SubtypeOf(positiveBase) {
		t.Fatal("nonnegative integers should not be a subtype of positive integers")
	}
	if positiveBase.Equivalent(nonnegative) {
		t.Fatal("different accepted value sets should not be equivalent")
	}
}

func TestCrossCompilerSumRelations(t *testing.T) {
	t.Parallel()

	positive := Where(Int(), Binary(X(), Gt, IntExpr(0)))
	negative := Where(Int(), Binary(X(), Lt, IntExpr(0)))
	states := Where(String(), Binary(X(), In, ListExpr(LitString("ready"), LitString("done"))))
	type testcase struct {
		name   string
		a, b   ks.Value
		wantAB bool
		wantBA bool
	}
	for _, tc := range []testcase{
		{
			name:   "bounded integer and string",
			a:      Sum(Int8(), String()),
			b:      Sum(String(), Int8()),
			wantAB: true,
			wantBA: true,
		},
		{
			name:   "disjoint ranges",
			a:      Sum(positive, negative),
			b:      Sum(negative, positive),
			wantAB: true,
			wantBA: true,
		},
		{
			name:   "enum and integer",
			a:      Sum(states, Int()),
			b:      Sum(Int(), states),
			wantAB: true,
			wantBA: true,
		},
		{
			name:   "nested union with metadata",
			a:      Object(Field("choice", With(Sum(Int8(), String()), Attr("owner", StrExpr("a"))))),
			b:      Object(Field("choice", With(Sum(String(), Int8()), Attr("owner", StrExpr("b"))))),
			wantAB: true,
			wantBA: true,
		},
		{
			name:   "constrained member belongs to union",
			a:      Int8(),
			b:      Sum(Int8(), String()),
			wantAB: true,
		},
		{
			name:   "wider range is not equivalent",
			a:      Sum(Int8(), String()),
			b:      Sum(Int16(), String()),
			wantAB: true,
		},
		{
			name:   "unconstrained integer is not bounded",
			a:      Sum(Int(), String()),
			b:      Sum(Int8(), String()),
			wantBA: true,
		},
		{
			name: "different alternatives are not equivalent",
			a:    Sum(Int8(), Bool()),
			b:    Sum(Int8(), String()),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			aCompiler, bCompiler := katschema.NewCompiler(), katschema.NewCompiler()
			// NOTE(i4k): this explore an implementation detail.
			// Here we shift local IDs so matching definitions cannot rely on matching IDs.
			compile(t, bCompiler, Object(Field("padding", List(String()))))
			a, b := compile(t, aCompiler, tc.a), compile(t, bCompiler, tc.b)
			if got := a.SubtypeOf(b); got != tc.wantAB {
				t.Errorf("a.SubtypeOf(b) = %v, want %v", got, tc.wantAB)
			}
			if got := b.SubtypeOf(a); got != tc.wantBA {
				t.Errorf("b.SubtypeOf(a) = %v, want %v", got, tc.wantBA)
			}
			wantEquivalent := tc.wantAB && tc.wantBA
			if got := a.Equivalent(b); got != wantEquivalent {
				t.Errorf("a.Equivalent(b) = %v, want %v", got, wantEquivalent)
			}
			if got := b.Equivalent(a); got != wantEquivalent {
				t.Errorf("b.Equivalent(a) = %v, want %v", got, wantEquivalent)
			}
		})
	}
}

func TestOverlay(t *testing.T) {
	t.Parallel()

	cc := katschema.NewCompiler()
	base, err := cc.Compile(t.Context(), Object(
		Field("id", With(String(), Flag("pk"))),
		Field("text", Optional(String())),
	))
	if err != nil {
		t.Fatal(err)
	}
	top, err := cc.Compile(t.Context(), Object(
		Field("custom_fields", Object(Field("priority", Int()))),
		Field("text", String()),
	))
	if err != nil {
		t.Fatal(err)
	}
	typ, err := katschema.Overlay(base, top)
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

	nested, err := katschema.Overlay(typ, top)
	if err != nil || nested.SemanticFingerprint() != typ.SemanticFingerprint() {
		t.Fatalf("nested overlay = %v, want equivalent effective type", err)
	}
}

func TestOverlayAcrossCompilers(t *testing.T) {
	t.Parallel()

	baseCompiler := katschema.NewCompiler()
	topCompiler := katschema.NewCompilerWithResolver(katschema.ResolverFunc(
		func(_ context.Context, name string) (katschema.TypeResolution, error) {
			if name != "positive_int" {
				return katschema.TypeResolution{}, katschema.ErrUnknownType
			}
			return katschema.TypeResolution{
				Value: With(Int(), Check(Binary(X(), Gt, IntExpr(0)))),
				Name:  name,
			}, nil
		},
	))
	// Deliberately compile unrelated types first so equivalent definitions do
	// not happen to have matching arena-local IDs.
	if _, err := topCompiler.Compile(t.Context(), Object(
		Field("padding", Sum(String(), List(Int()))),
		Field("other", With(Real(), Check(Binary(X(), Ge, DecimalExpr("0.1"))))),
	)); err != nil {
		t.Fatal(err)
	}

	base, err := baseCompiler.Compile(t.Context(), Object(
		Field("id", With(String(), Flag("pk"))),
		Field("replace", Optional(With(
			Int(),
			Check(Binary(X(), Ge, IntExpr(0))),
			Attr("source", StrExpr("core")),
			Flag("core_only"),
		))),
		Field("nested", Object(Field("core", String()))),
		Field("custom_fields", Object(Field("legacy", String()))),
	))
	if err != nil {
		t.Fatal(err)
	}
	top, err := topCompiler.Compile(t.Context(), Object(
		Field("replace", With(
			Int(),
			Check(Binary(X(), Ge, IntExpr(5))),
			Check(Binary(X(), Le, IntExpr(10))),
			Attr("source", StrExpr("request")),
			Attr("tags", ListExpr(LitString("dynamic"), LitString("scoped"))),
		)),
		Field("nested", Object(Field("request", String()))),
		Field("custom_fields", Object(Field("priority", With(
			Int(), Check(Binary(X(), Gt, IntExpr(0))),
		)))),
		Field("items", List(With(Int(), Check(Binary(X(), Gt, IntExpr(0)))))),
		Field("choice", Sum(String(), Int())),
		Field("ratio", Optional(With(Float32(), Check(Binary(X(), Lt, DecimalExpr("10")))))),
		Field("tuple", List(LitString("version"), LitInt(2))),
		Field("state", With(String(), Check(Binary(X(), In, ListExpr(LitString("ready"), LitString("done")))))),
		Field("rank", Type("positive_int")),
	))
	if err != nil {
		t.Fatal(err)
	}

	baseDefinition := base.DefinitionFingerprint()
	topDefinition := top.DefinitionFingerprint()
	overlay, err := katschema.Overlay(base, top)
	if err != nil {
		t.Fatal(err)
	}
	if base.DefinitionFingerprint() != baseDefinition || top.DefinitionFingerprint() != topDefinition {
		t.Fatal("overlay mutated an input type")
	}

	replaced, ok := overlay.Field("replace")
	if !ok || replaced.Optional {
		t.Fatalf("replace field = %+v, found=%v; top definition should replace the complete base field", replaced, ok)
	}
	if got, ok := replaced.Metadata().Get("source"); !ok || got != "request" {
		t.Fatalf("replace source metadata = %v, found=%v", got, ok)
	}
	if _, ok := replaced.Metadata().Get("core_only"); ok {
		t.Fatal("metadata from the replaced field definition leaked from the base")
	}
	if got, ok := replaced.Metadata().Get("tags"); !ok {
		t.Fatal("replace tags metadata is missing")
	} else if diff := cmp.Diff(got, []any{"dynamic", "scoped"}); diff != "" {
		t.Fatalf("replace tags metadata differs (-got +want):\n%s", diff)
	}
	if got, ok := overlay.Traverse(katschema.Path{"rank"}); !ok || got.Name() != "positive_int" {
		t.Fatalf("resolved type name = %q, want positive_int", got.Name())
	}
	if _, ok := overlay.Traverse(katschema.Path{"custom_fields", "legacy"}); ok {
		t.Fatal("top custom_fields definition did not replace the complete base field")
	}
	if got, ok := overlay.Traverse(katschema.Path{"custom_fields", "priority"}); !ok || got.Kind() != katschema.Refined {
		t.Fatalf("custom_fields.priority = %v, found=%v; want refinement", got.Kind(), ok)
	}
	if got, ok := overlay.Traverse(katschema.Path{"nested", "request"}); !ok || got.Kind() != katschema.String {
		t.Fatalf("nested.request = %v, found=%v", got.Kind(), ok)
	}
	if _, ok := overlay.Traverse(katschema.Path{"nested", "core"}); ok {
		t.Fatal("top nested definition did not replace the complete base field")
	}

	valid := Object(
		Field("id", LitString("one")),
		Field("replace", LitInt(7)),
		Field("nested", Object(Field("request", LitString("ok")))),
		Field("custom_fields", Object(Field("priority", LitInt(1)))),
		Field("items", List(LitInt(1), LitInt(2))),
		Field("choice", LitString("text")),
		Field("ratio", LitInt(1)),
		Field("tuple", List(LitString("version"), LitInt(2))),
		Field("state", LitString("ready")),
		Field("rank", LitInt(1)),
	)
	for _, tc := range []struct {
		name  string
		value ks.Value
	}{
		{"id", LitString("one")},
		{"replace", LitInt(7)},
		{"nested", Object(Field("request", LitString("ok")))},
		{"custom_fields", Object(Field("priority", LitInt(1)))},
		{"items", List(LitInt(1), LitInt(2))},
		{"choice", LitString("text")},
		{"ratio", LitInt(1)},
		{"tuple", List(LitString("version"), LitInt(2))},
		{"state", LitString("ready")},
		{"rank", LitInt(1)},
	} {
		field, ok := overlay.Field(tc.name)
		if !ok {
			t.Fatalf("missing field %q", tc.name)
		}
		if err := field.Type.Validate(tc.value); err != nil {
			t.Errorf("field %q rejected valid value: %v", tc.name, err)
		}
	}
	ratio, _ := overlay.Field("ratio")
	if err := ratio.Type.Validate(LitInt(11)); err == nil {
		t.Fatal("overlaid float range accepted an out-of-range value")
	}
	if err := overlay.Validate(valid); err != nil {
		t.Fatalf("valid cross-compiler overlay value rejected: %v", err)
	}
	for _, invalid := range []ks.Value{
		Object(
			Field("id", LitString("one")),
			Field("replace", LitInt(2)),
			Field("nested", Object(Field("request", LitString("ok")))),
			Field("custom_fields", Object(Field("priority", LitInt(1)))),
			Field("items", List(LitInt(1))),
			Field("choice", LitString("text")),
			Field("tuple", List(LitString("version"), LitInt(2))),
			Field("state", LitString("ready")),
			Field("rank", LitInt(1)),
		),
		Object(
			Field("id", LitString("one")),
			Field("replace", LitInt(11)),
			Field("nested", Object(Field("request", LitString("ok")))),
			Field("custom_fields", Object(Field("priority", LitInt(1)))),
			Field("items", List(LitInt(1))),
			Field("choice", LitString("text")),
			Field("tuple", List(LitString("version"), LitInt(2))),
			Field("state", LitString("ready")),
			Field("rank", LitInt(1)),
		),
		Object(
			Field("id", LitString("one")),
			Field("replace", LitInt(7)),
			Field("nested", Object(Field("request", LitString("ok")))),
			Field("custom_fields", Object(Field("priority", LitInt(0)))),
			Field("items", List(LitInt(1))),
			Field("choice", LitString("text")),
			Field("tuple", List(LitString("version"), LitInt(2))),
			Field("state", LitString("ready")),
			Field("rank", LitInt(1)),
		),
		Object(
			Field("id", LitString("one")),
			Field("replace", LitInt(7)),
			Field("nested", Object(Field("request", LitString("ok")))),
			Field("custom_fields", Object(Field("priority", LitInt(1)))),
			Field("items", List(LitInt(0))),
			Field("choice", LitString("text")),
			Field("tuple", List(LitString("version"), LitInt(2))),
			Field("state", LitString("unknown")),
			Field("rank", LitInt(1)),
		),
		Object(
			Field("id", LitString("one")),
			Field("replace", LitInt(7)),
			Field("nested", Object(Field("request", LitString("ok")))),
			Field("custom_fields", Object(Field("priority", LitInt(1)))),
			Field("items", List(LitInt(1))),
			Field("choice", LitString("text")),
			Field("tuple", List(LitString("version"), LitInt(2))),
			Field("state", LitString("ready")),
			Field("rank", LitInt(0)),
		),
	} {
		if err := overlay.Validate(invalid); err == nil {
			t.Fatal("invalid value accepted by cross-compiler overlay")
		}
	}

	gotCustomFields, _ := overlay.Traverse(katschema.Path{"custom_fields"})
	wantCustomFields, _ := top.Traverse(katschema.Path{"custom_fields"})
	if !gotCustomFields.Equivalent(wantCustomFields) {
		t.Fatal("overlaid custom-fields type should be semantically equivalent across arenas")
	}
}

func TestOverlaySemanticOperationsAcrossCompilers(t *testing.T) {
	t.Parallel()

	baseCompiler := katschema.NewCompiler()
	topCompiler := katschema.NewCompiler()
	expectedCompiler := katschema.NewCompiler()
	if _, err := topCompiler.Compile(t.Context(), Object(
		Field("padding", Sum(String(), List(Int()))),
	)); err != nil {
		t.Fatal(err)
	}

	base, err := baseCompiler.Compile(t.Context(), Object(
		Field("id", String()),
		Field("replace", String()),
		Field("retained", Optional(Int())),
	))
	if err != nil {
		t.Fatal(err)
	}
	top, err := topCompiler.Compile(t.Context(), Object(
		Field("replace", Optional(Int())),
		Field("added", Bool()),
	))
	if err != nil {
		t.Fatal(err)
	}
	want, err := expectedCompiler.Compile(t.Context(), Object(
		Field("id", String()),
		Field("replace", Optional(Int())),
		Field("retained", Optional(Int())),
		Field("added", Bool()),
	))
	if err != nil {
		t.Fatal(err)
	}
	overlay, err := katschema.Overlay(base, top)
	if err != nil {
		t.Fatal(err)
	}
	if !overlay.Equivalent(want) || !want.Equivalent(overlay) {
		t.Fatal("overlay should be semantically equivalent to the expected object across arenas")
	}
	if !overlay.SubtypeOf(want) || !want.SubtypeOf(overlay) {
		t.Fatal("overlay should compare as a subtype in both directions with the expected object")
	}
	if overlay.SemanticFingerprint() != want.SemanticFingerprint() {
		t.Fatal("overlay semantic fingerprint differs from the expected object")
	}
	scalar, err := katschema.Compile(t.Context(), Int())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := katschema.Overlay(scalar, base); err == nil {
		t.Fatal("overlay should reject a non-object operand")
	}
}

func TestOverlayConcurrentReadOnlyOperations(t *testing.T) {
	t.Parallel()

	base, err := katschema.Compile(t.Context(), Object(Field("core", String())))
	if err != nil {
		t.Fatal(err)
	}
	top, err := katschema.Compile(t.Context(), Object(Field("dynamic", List(Int()))))
	if err != nil {
		t.Fatal(err)
	}
	want, err := katschema.Compile(t.Context(), Object(
		Field("core", String()),
		Field("dynamic", List(Int())),
	))
	if err != nil {
		t.Fatal(err)
	}
	overlay, err := katschema.Overlay(base, top)
	if err != nil {
		t.Fatal(err)
	}
	value := Object(
		Field("core", LitString("ok")),
		Field("dynamic", List(LitInt(1), LitInt(2))),
	)

	const workers = 8
	const iterations = 20
	var wg sync.WaitGroup
	errs := make(chan error, workers*iterations*4)
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range iterations {
				if _, ok := overlay.Traverse(katschema.Path{"dynamic"}); !ok {
					errs <- errors.New("Traverse failed on a concurrent overlay read")
				}
				if err := overlay.Validate(value); err != nil {
					errs <- err
				}
				if !overlay.Equivalent(want) {
					errs <- errors.New("Equivalent failed on a concurrent overlay read")
				}
				if overlay.SemanticFingerprint() != want.SemanticFingerprint() {
					errs <- errors.New("fingerprint changed on a concurrent overlay read")
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

func TestMetadataIsPreserved(t *testing.T) {
	t.Parallel()

	cc := katschema.NewCompiler()
	withOptions, err := cc.Compile(t.Context(), Object(
		Field("id", With(String(), Flag("pk"), Attr("owner", StrExpr("ingestion")))),
	))
	if err != nil {
		t.Fatal(err)
	}
	withoutOptions, err := cc.Compile(t.Context(), Object(Field("id", String())))
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

func TestNullMetadata(t *testing.T) {
	t.Parallel()
	typ, err := katschema.Compile(t.Context(), With(String(),
		Attr("empty", NullExpr()),
		Attr("items", ListExpr(LitNull(), LitString("value"))),
	))
	if err != nil {
		t.Fatal(err)
	}
	metadata := typ.Metadata()
	if value, ok := metadata.Get("empty"); !ok || value != nil {
		t.Fatalf("null metadata = %v, found=%v; want nil, true", value, ok)
	}
	if value, ok := metadata.Get("items"); !ok {
		t.Fatal("list containing null metadata is missing")
	} else if diff := cmp.Diff([]any{nil, "value"}, value); diff != "" {
		t.Fatalf("metadata differs (-want +got):\n%s", diff)
	}
}

func TestResolver(t *testing.T) {
	t.Parallel()

	resolverCalls := 0
	resolver := katschema.ResolverFunc(func(_ context.Context, name string) (katschema.TypeResolution, error) {
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
	typ, err := compiler.Compile(t.Context(), Type("analyzed"))
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
	if _, err := compiler.Compile(t.Context(), Type("analyzed")); err != nil {
		t.Fatal(err)
	}
	if resolverCalls != 2 {
		t.Fatalf("resolver calls=%d, want one call per compile to obtain the scope key", resolverCalls)
	}
	optionalField, err := compiler.Compile(t.Context(), Object(Field("custom_fields", With(
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
	customerCompiler := katschema.NewCompilerWithResolver(katschema.ResolverFunc(func(_ context.Context, name string) (katschema.TypeResolution, error) {
		return katschema.TypeResolution{
			Value:    With(String(), Attr("customer", StrExpr(customer))),
			CacheKey: "custom_fields:" + customer,
		}, nil
	}))
	abc, err := customerCompiler.Compile(t.Context(), Type("custom_fields"))
	if err != nil {
		t.Fatal(err)
	}
	customer = "xyz"
	xyz, err := customerCompiler.Compile(t.Context(), Type("custom_fields"))
	if err != nil {
		t.Fatal(err)
	}
	if abc.DefinitionFingerprint() == xyz.DefinitionFingerprint() {
		t.Fatal("customer-scoped cache keys must not reuse another customer's definition")
	}

	_, err = katschema.CompileWithResolver(t.Context(), Type("missing"), resolver)
	if err == nil || !errors.Is(err, katschema.ErrUnknownType) {
		t.Fatalf("missing type error=%v, want ErrUnknownType", err)
	}
}

func TestTypeNameIsOptionalAndNotSemantic(t *testing.T) {
	t.Parallel()

	// NOTE(i4k): this is not the right way of defining a resolver.
	// For testing purposes this wraps every non-builtin name with a test constraint.
	resolver := katschema.ResolverFunc(func(_ context.Context, name string) (katschema.TypeResolution, error) {
		return katschema.TypeResolution{
			Value: With(Int(), Check(Binary(X(), Gt, IntExpr(0)))),
			Name:  name,
		}, nil
	})
	compiler := katschema.NewCompilerWithResolver(resolver)

	positiveInt, err := compiler.Compile(t.Context(), Type("positive_int"))
	if err != nil {
		t.Fatal(err)
	}
	if got := positiveInt.Name(); got != "positive_int" {
		t.Fatalf("resolved type name=%q, want positive_int", got)
	}
	bounded, err := compiler.Compile(t.Context(), With(
		Type("positive_int"),
		Check(Binary(X(), Lt, IntExpr(100))),
	))
	if err != nil {
		t.Fatal(err)
	}
	if got := bounded.Name(); got != "positive_int" {
		t.Fatalf("refined resolved type name=%q, want positive_int", got)
	}

	score, err := compiler.Compile(t.Context(), Type("score"))
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

	unnamed, err := katschema.Compile(t.Context(), With(Int(), Check(Binary(X(), Gt, IntExpr(0)))))
	if err != nil {
		t.Fatal(err)
	}
	if got := unnamed.Name(); got != "" {
		t.Fatalf("unnamed refinement name=%q, want empty", got)
	}
}

func TestResolverRejectsRecursion(t *testing.T) {
	t.Parallel()

	resolver := katschema.ResolverFunc(func(_ context.Context, name string) (katschema.TypeResolution, error) {
		return katschema.TypeResolution{Value: Type(map[string]string{"a": "b", "b": "a"}[name])}, nil
	})
	_, err := katschema.CompileWithResolver(t.Context(), Type("a"), resolver)
	if err == nil || !errors.Is(err, katschema.ErrResolveCycle) {
		t.Fatalf("recursive resolver error=%v", err)
	}
}

func TestUnknownTypeFromCompiler(t *testing.T) {
	t.Parallel()

	_, err := katschema.Compile(t.Context(), Type("missing"))
	if err == nil || !errors.Is(err, katschema.ErrUnknownType) {
		t.Fatalf("unknown type error=%v, want ErrUnknownType", err)
	}
}

func TestCompileContextIsPassedToResolver(t *testing.T) {
	t.Parallel()

	type contextKey struct{}
	key := contextKey{}
	ctx := context.WithValue(context.Background(), key, "request-value")
	resolver := katschema.ResolverFunc(func(got context.Context, name string) (katschema.TypeResolution, error) {
		if name != "request_type" {
			return katschema.TypeResolution{}, katschema.ErrUnknownType
		}
		if got.Value(key) != "request-value" {
			t.Fatalf("resolver context value=%v, want request-value", got.Value(key))
		}
		return katschema.TypeResolution{Value: String()}, nil
	})

	if _, err := katschema.CompileWithResolver(ctx, Type("request_type"), resolver); err != nil {
		t.Fatal(err)
	}
}

func TestResolverPreservesConstrainedType(t *testing.T) {
	t.Parallel()

	resolver := katschema.ResolverFunc(func(_ context.Context, name string) (katschema.TypeResolution, error) {
		if name != "language" {
			return katschema.TypeResolution{}, katschema.ErrUnknownType
		}
		return katschema.TypeResolution{Value: With(
			String(),
			Check(Binary(X(), In, ListExpr(LitString("en"), LitString("pt")))),
			Flag("logical"),
		)}, nil
	})
	typ, err := katschema.CompileWithResolver(t.Context(), Object(Field("language", Type("language"))), resolver)
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
