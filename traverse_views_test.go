package katschema_test

import (
	"context"
	"testing"

	"github.com/birdie-ai/katschema"
	"github.com/birdie-ai/katschema/ks"
)

func TestTraverseRetainsSourceTypes(t *testing.T) {
	t.Parallel()
	core := compileText(t, katschema.NewCompiler(), `([
		({
			"id": (string, pk),
			"custom_fields": ({"legacy": (string)}, optional)
		}, owner="core")
	], collection=true)`)
	request := katschema.NewCompilerWithResolver(katschema.ResolverFunc(
		func(_ context.Context, name string) (katschema.TypeResolution, error) {
			if name != "timestamp" {
				return katschema.TypeResolution{}, katschema.ErrUnknownType
			}
			return katschema.TypeResolution{Name: "timestamp", Value: ks.String()}, nil
		},
	))
	patch := compileText(t, request, `{
		"custom_fields": ({
			"priority": (int, x > 0, mapping="numbers"),
			"timestamp": (timestamp),
			"tags": [({"name": (string)}, owner="request")]
		}, optional)
	}`)
	overlay, err := katschema.Overlay(core, patch)
	if err != nil {
		t.Fatal(err)
	}
	coreID, _ := core.Traverse(katschema.Path{"id"})
	priority, _ := patch.Traverse(katschema.Path{"custom_fields", "priority"})
	timestamp, _ := patch.Traverse(katschema.Path{"custom_fields", "timestamp"})
	tags, _ := patch.Traverse(katschema.Path{"custom_fields", "tags"})
	tagName, _ := patch.Traverse(katschema.Path{"custom_fields", "tags", "name"})
	type testcase struct {
		name  string
		path  katschema.Path
		want  katschema.Type
		found bool
	}
	for _, tc := range []testcase{
		{
			name:  "empty path preserves discovered root wrappers",
			want:  overlay,
			found: true,
		},
		{
			name:  "static child keep core arena",
			path:  katschema.Path{"id"},
			want:  coreID,
			found: true,
		},
		{
			name:  "refined child keep request arena",
			path:  katschema.Path{"custom_fields", "priority"},
			want:  priority,
			found: true,
		},
		{
			name:  "named child keep request arena",
			path:  katschema.Path{"custom_fields", "timestamp"},
			want:  timestamp,
			found: true,
		},
		{
			name:  "terminal list is not unwrapped",
			path:  katschema.Path{"custom_fields", "tags"},
			want:  tags,
			found: true,
		},
		{
			name:  "traverse nested request list and refinement",
			path:  katschema.Path{"custom_fields", "tags", "name"},
			want:  tagName,
			found: true,
		},
		{
			name: "replaced field is absent",
			path: katschema.Path{"custom_fields", "legacy"},
		},
		{
			name: "scalar cannot be traversed",
			path: katschema.Path{"custom_fields", "priority", "missing"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := overlay.Traverse(tc.path)
			if ok != tc.found || got != tc.want {
				t.Fatalf("Traverse(%v) = %+v, %v; want original handle %+v, %v", tc.path, got, ok, tc.want, tc.found)
			}
		})
	}
	if timestamp.Name() != "timestamp" || priority.Validate(ks.LitInt(-1)) == nil {
		t.Fatal("traversal lost a name or refinement")
	}
	if mapping, _ := priority.Metadata().Get("mapping"); mapping != "numbers" {
		t.Fatal("traversal lost field type metadata")
	}
	newPatch := compileText(t, katschema.NewCompiler(), `{"priority": (string)}`)
	updated, err := katschema.OverlayAt(overlay, katschema.Path{"custom_fields"}, newPatch)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := updated.Traverse(katschema.Path{"custom_fields", "priority"})
	want, _ := newPatch.Traverse(katschema.Path{"priority"})
	if !ok || got != want {
		t.Fatal("nested overlay lost the latest source type")
	}
	unchanged, _ := overlay.Traverse(katschema.Path{"custom_fields", "priority"})
	if unchanged != priority {
		t.Fatal("nested overlay mutated the previous view")
	}
}
