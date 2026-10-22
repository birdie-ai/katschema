package katschema_test

import (
	"context"
	"testing"

	"github.com/birdie-ai/katschema"
	"github.com/birdie-ai/katschema/ks"
	"github.com/google/go-cmp/cmp"
)

func compile(t testing.TB, cc *katschema.Compiler, value ks.Value) katschema.Type {
	t.Helper()
	typ, err := cc.Compile(t.Context(), value)
	if err != nil {
		t.Fatal(err)
	}
	return typ
}

func TestOverlayPreservesEntityWrappers(t *testing.T) {
	t.Parallel()

	pks := ks.Attr("pks", ks.ListExpr(ks.LitString("id")))
	listSchema := ks.With(
		ks.List(ks.Type("record")),
		ks.Check(ks.Binary(ks.Funcall("len", ks.X()), ks.Le, ks.IntExpr(2))),
		ks.Attr("scope", ks.StrExpr("core")),
	)
	recordCompiler := func(custom ks.Value) *katschema.Compiler {
		return katschema.NewCompilerWithResolver(katschema.ResolverFunc(
			func(_ context.Context, name string) (katschema.TypeResolution, error) {
				if name != "record" {
					return katschema.TypeResolution{}, katschema.ErrUnknownType
				}
				return katschema.TypeResolution{
					Value: ks.With(ks.Object(
						ks.Field("id", ks.With(ks.String(), ks.Flag("pk"))),
						ks.Field("custom_fields", ks.Optional(custom)),
					), pks),
					Name: "record",
				}, nil
			},
		))
	}
	originalFields := ks.Object(ks.Field("legacy", ks.Optional(ks.String())))
	dynamicFields := ks.Object(ks.Field("priority", ks.Optional(ks.Int())))
	core := compile(t, recordCompiler(originalFields), listSchema)
	coreFingerprint := core.Fingerprint()
	top := compile(t, katschema.NewCompiler(), ks.Object(
		ks.Field("custom_fields", ks.Optional(dynamicFields)),
	))
	got, err := katschema.Overlay(core, top)
	if err != nil {
		t.Fatal(err)
	}
	want := compile(t, recordCompiler(dynamicFields), listSchema)
	if got.Kind() != katschema.Refined {
		t.Fatalf("root kind = %s, want refined list", got.Kind())
	}
	if scope, ok := got.Metadata().Get("scope"); !ok || scope != "core" {
		t.Fatalf("list metadata = %v, found=%v", scope, ok)
	}
	list, ok := got.Base()
	if !ok || list.Kind() != katschema.List {
		t.Fatalf("root base kind = %s, found=%v", list.Kind(), ok)
	}
	record, ok := list.Element()
	if !ok || record.Name() != "record" {
		t.Fatalf("element name = %q, found=%v", record.Name(), ok)
	}
	if value, ok := record.Metadata().Get("pks"); !ok {
		t.Fatal("record pks metadata is missing")
	} else if diff := cmp.Diff(value, []any{"id"}); diff != "" {
		t.Fatalf("record pks differs (-got +want):\n%s", diff)
	}
	if !got.Equivalent(want) || got.Fingerprint() != want.Fingerprint() || got.SemanticFingerprint() != want.SemanticFingerprint() {
		t.Fatal("wrapped overlay differs from independently compiled effective definition")
	}
	if _, ok := got.Traverse(katschema.Path{"custom_fields", "legacy"}); ok {
		t.Fatal("replaced custom fields remain visible")
	}
	if _, ok := got.Traverse(katschema.Path{"custom_fields", "priority"}); !ok {
		t.Fatal("dynamic field is missing through list/refinement wrappers")
	}

	recordValue := ks.Object(
		ks.Field("id", ks.LitString("one")),
		ks.Field("custom_fields", ks.Object(ks.Field("priority", ks.LitInt(3)))),
	)

	type testcase struct {
		name  string
		value ks.Value
		valid bool
	}

	for _, tc := range []testcase{
		{
			name:  "valid collection",
			value: ks.List(recordValue),
			valid: true,
		},
		{
			name:  "optional custom fields",
			value: ks.List(ks.Object(ks.Field("id", ks.LitString("one")))),
			valid: true,
		},
		{
			name:  "list shape is preserved",
			value: recordValue,
		},
		{
			name:  "list constraint is preserved",
			value: ks.List(recordValue, recordValue, recordValue),
		},
		{
			name:  "required core field",
			value: ks.List(ks.Object()),
		},
		{
			name: "invalid dynamic field",
			value: ks.List(ks.Object(
				ks.Field("id", ks.LitString("one")),
				ks.Field("custom_fields", ks.Object(ks.Field("priority", ks.LitString("wrong")))),
			)),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := got.Validate(tc.value); (err == nil) != tc.valid {
				t.Fatalf("Validate = %v, want valid=%v", err, tc.valid)
			}
		})
	}
	empty := compile(t, katschema.NewCompiler(), ks.Object())
	repeated, err := katschema.Overlay(got, empty)
	if err != nil || repeated.Fingerprint() != got.Fingerprint() {
		t.Fatalf("empty repeated overlay changed wrapped definition: %v", err)
	}
	if core.Fingerprint() != coreFingerprint {
		t.Fatal("overlay changed shared core")
	}
}

func TestFingerprintIndependentOfCompilationOrder(t *testing.T) {
	t.Parallel()

	choice := ks.Sum(ks.LitString("a"), ks.LitString("b"))
	a := compile(t, katschema.NewCompiler(), ks.Object(
		ks.Field("choice", choice),
		ks.Field("anchor", ks.LitString("b")),
		ks.Field("tag", ks.With(ks.String(), ks.Attr("owner", ks.StrExpr("core")))),
	))
	b := compile(t, katschema.NewCompiler(), ks.Object(
		ks.Field("anchor", ks.LitString("b")),
		ks.Field("tag", ks.With(ks.String(), ks.Attr("owner", ks.StrExpr("core")))),
		ks.Field("choice", choice),
	))
	if !a.Equivalent(b) || a.Fingerprint() != b.Fingerprint() || a.SemanticFingerprint() != b.SemanticFingerprint() {
		t.Fatal("field/union compilation order changed identity")
	}
	empty := compile(t, katschema.NewCompiler(), ks.Object())
	overlay, err := katschema.Overlay(a, empty)
	if err != nil {
		t.Fatal(err)
	}
	if overlay.Fingerprint() != a.Fingerprint() || overlay.SemanticFingerprint() != a.SemanticFingerprint() {
		t.Fatal("empty overlay changed fingerprints")
	}
	base := compile(t, katschema.NewCompiler(), ks.Object(
		ks.Field("anchor", ks.LitString("b")),
		ks.Field("tag", ks.With(ks.String(), ks.Attr("owner", ks.StrExpr("core")))),
	))
	top := compile(t, katschema.NewCompiler(), ks.Object(ks.Field("choice", choice)))
	layered, err := katschema.Overlay(base, top)
	if err != nil || layered.Fingerprint() != a.Fingerprint() {
		t.Fatalf("effective overlay fingerprint differs: %v", err)
	}
}

func TestOverlayKeepsObjectConstraints(t *testing.T) {
	t.Parallel()
	core := compile(t, katschema.NewCompiler(), ks.With(
		ks.Object(ks.Field("id", ks.String())),
		ks.Check(ks.Binary(ks.Funcall("len", ks.X()), ks.Le, ks.IntExpr(1))),
	))
	top := compile(t, katschema.NewCompiler(), ks.Object(ks.Field("new", ks.Optional(ks.Int()))))
	got, err := katschema.Overlay(core, top)
	if err != nil {
		t.Fatal(err)
	}
	if err := got.Validate(ks.Object(ks.Field("id", ks.LitString("one")))); err != nil {
		t.Fatal(err)
	}
	if err := got.Validate(ks.Object(ks.Field("id", ks.LitString("one")), ks.Field("new", ks.LitInt(1)))); err == nil {
		t.Fatal("overlay dropped the object's length constraint")
	}
}

func TestOverlayPatchWrapperPolicy(t *testing.T) {
	t.Parallel()
	core := compile(t, katschema.NewCompiler(), ks.List(ks.With(
		ks.Object(ks.Field("id", ks.String())),
		ks.Attr("owner", ks.StrExpr("core")),
	)))
	top := compile(t, katschema.NewCompiler(), ks.List(ks.With(
		ks.Object(ks.Field("new", ks.Optional(ks.Int()))),
		ks.Attr("owner", ks.StrExpr("patch")),
	)))
	got, err := katschema.Overlay(core, top)
	if err != nil {
		t.Fatal(err)
	}
	element, _ := got.Element()
	if owner, ok := element.Metadata().Get("owner"); !ok || owner != "core" {
		t.Fatalf("patch root annotations replaced core configuration: %v, found=%v", owner, ok)
	}
	constrained := compile(t, katschema.NewCompiler(), ks.With(
		ks.Object(ks.Field("new", ks.Optional(ks.Int()))),
		ks.Check(ks.Binary(ks.Funcall("len", ks.X()), ks.Le, ks.IntExpr(1))),
	))
	if _, err := katschema.Overlay(core, constrained); err == nil {
		t.Fatal("constrained patch root was silently discarded")
	}
}

func TestOverlayAtPreservesAncestors(t *testing.T) {
	t.Parallel()
	definition := func(custom ks.Value) ks.Value {
		return ks.List(ks.With(ks.Object(
			ks.Field("id", ks.String()),
			ks.Field("children", ks.With(ks.List(ks.Object(
				ks.Field("id", ks.Int()),
				ks.Field("custom_fields", ks.Optional(custom)),
			)), ks.Flag("optional"), ks.Flag("nested"))),
		), ks.Attr("owner", ks.StrExpr("core"))))
	}
	core := compile(t, katschema.NewCompiler(), definition(ks.Object(
		ks.Field("old", ks.Optional(ks.String())),
	)))
	dynamic := ks.Object(ks.Field("priority", ks.Optional(ks.Int())))
	patch := compile(t, katschema.NewCompiler(), ks.Object(
		ks.Field("custom_fields", ks.Optional(dynamic)),
	))
	got, err := katschema.OverlayAt(core, katschema.Path{"children"}, patch)
	if err != nil {
		t.Fatal(err)
	}
	want := compile(t, katschema.NewCompiler(), definition(dynamic))
	if !got.Equivalent(want) || got.Fingerprint() != want.Fingerprint() || got.SemanticFingerprint() != want.SemanticFingerprint() {
		t.Fatal("nested overlay differs from a complete compiled definition")
	}
	before, _ := core.Traverse(katschema.Path{"children", "id"})
	after, _ := got.Traverse(katschema.Path{"children", "id"})
	if before != after {
		t.Fatal("unchanged child type was rebuilt")
	}
	list, _ := got.Element()
	object, _ := list.Base()
	children, _ := object.Field("children")
	if !children.Optional || !children.Metadata().Has("nested") {
		t.Fatal("ancestor field flags were lost")
	}
	if _, ok := got.Traverse(katschema.Path{"children", "custom_fields", "old"}); ok {
		t.Fatal("replaced child fields leaked")
	}
	if _, ok := core.Traverse(katschema.Path{"children", "custom_fields", "old"}); !ok {
		t.Fatal("nested overlay mutated the source")
	}
	for _, tc := range []struct {
		value ks.Value
		valid bool
	}{
		{
			value: ks.List(ks.Object(ks.Field("id", ks.LitString("parent")), ks.Field("children", ks.List(ks.Object(
				ks.Field("id", ks.LitInt(1)),
				ks.Field("custom_fields", ks.Object(ks.Field("priority", ks.LitInt(2)))),
			))))),
			valid: true,
		},
		{
			value: ks.List(ks.Object(ks.Field("id", ks.LitString("parent")), ks.Field("children", ks.List(ks.Object(
				ks.Field("id", ks.LitInt(1)),
				ks.Field("custom_fields", ks.Object(ks.Field("priority", ks.LitString("bad")))),
			))))),
		},
	} {
		if err := got.Validate(tc.value); (err == nil) != tc.valid {
			t.Fatalf("Validate = %v, want valid=%v", err, tc.valid)
		}
	}
	for _, path := range []katschema.Path{{"missing"}, {"children", "id"}} {
		if _, err := katschema.OverlayAt(core, path, patch); err == nil {
			t.Fatalf("OverlayAt(%v) succeeded for a missing/non-object target", path)
		}
	}
}
