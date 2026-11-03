package katschema_test

import (
	"errors"
	"testing"

	"github.com/birdie-ai/katschema"
	"github.com/birdie-ai/katschema/ks"
)

func TestValidateType(t *testing.T) {
	t.Parallel()

	schema := compileText(t, katschema.NewCompiler(), `{
		"id": (string, len(x) > 0),
		"tags": ([(string)], len(x) <= 2, optional)
	}`)
	type testcase struct {
		name  string
		value string
		valid bool
	}
	for _, tc := range []testcase{
		{name: "optional omitted", value: `{"id": "one"}`, valid: true},
		{name: "empty array", value: `{"id": "one", "tags": []}`, valid: true},
		{name: "array at limit", value: `{"id": "one", "tags": ["a", "b"]}`, valid: true},
		{name: "array over limit", value: `{"id": "one", "tags": ["a", "b", "c"]}`, valid: false},
		{name: "wrong element", value: `{"id": "one", "tags": [1]}`, valid: false},
		{name: "empty id", value: `{"id": ""}`, valid: false},
		{name: "missing id", value: "{}", valid: false},
		{name: "unknown field", value: `{"id": "one", "internal": 1}`, valid: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			value := compileText(t, katschema.NewCompiler(), tc.value)
			err := schema.ValidateType(value)
			if (err == nil) != tc.valid {
				t.Fatalf("ValidateType = %v, want valid=%v", err, tc.valid)
			}
			if err != nil && !errors.Is(err, katschema.ErrValidation) {
				t.Fatalf("error does not wrap ErrValidation: %v", err)
			}
		})
	}
}

func TestValidateTypeRequiresLiteral(t *testing.T) {
	t.Parallel()

	cc := katschema.NewCompiler()
	anyType := compileText(t, cc, "(any)")
	type testcase struct {
		name  string
		value katschema.Type
	}
	for _, tc := range []testcase{
		{name: "zero", value: katschema.Type{}},
		{name: "any", value: anyType},
		{name: "never", value: compileText(t, cc, "(never)")},
		{name: "int", value: compileText(t, cc, "(int)")},
		{name: "range", value: compileText(t, cc, "(int, x > 0)")},
		{name: "sum", value: compile(t, cc, ks.Sum(ks.LitInt(1), ks.LitInt(2)))},
		{name: "list schema", value: compileText(t, cc, "[(int)]")},
		{name: "object schema", value: compileText(t, cc, `{"id": (string)}`)},
		{name: "optional field", value: compileText(t, cc, `{"id": (int, x == 1, optional)}`)},
		{name: "nested schema", value: compileText(t, cc, `{"data": {"value": (any)}}`)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := anyType.ValidateType(tc.value); !errors.Is(err, katschema.ErrValidation) {
				t.Fatalf("ValidateType = %v, want ErrValidation", err)
			}
		})
	}
	value := compileText(t, cc, "1")
	if err := (katschema.Type{}).ValidateType(value); !errors.Is(err, katschema.ErrValidation) {
		t.Fatalf("zero schema ValidateType = %v, want ErrValidation", err)
	}
}

func TestValidateTypeOverlays(t *testing.T) {
	t.Parallel()

	input := compileText(t, katschema.NewCompiler(), `({"id": (string)}, entity)`)
	internal := compileText(t, katschema.NewCompiler(), `{"updated_at": (int)}`)
	schema, err := katschema.Overlay(input, internal)
	if err != nil {
		t.Fatal(err)
	}
	public := compileText(t, katschema.NewCompiler(), `({"id": "one"}, source)`)
	generated := compileText(t, katschema.NewCompiler(), `{"updated_at": 10}`)
	value, err := katschema.Overlay(public, generated)
	if err != nil {
		t.Fatal(err)
	}
	if err := schema.ValidateType(value); err != nil {
		t.Fatal(err)
	}
	if err := input.ValidateType(value); !errors.Is(err, katschema.ErrValidation) {
		t.Fatalf("input accepted generated field: %v", err)
	}
	if err := schema.ValidateType(public); !errors.Is(err, katschema.ErrValidation) {
		t.Fatalf("stored schema accepted missing generated field: %v", err)
	}
	if err := input.ValidateType(public); err != nil {
		t.Fatalf("original input changed: %v", err)
	}
}

func TestValidateTypeValueConstraints(t *testing.T) {
	t.Parallel()

	cc := katschema.NewCompiler()
	schema := compileText(t, cc, "(any)")
	value := compileText(t, cc, `({"id": 1}, len(x) <= 1)`)
	if err := schema.ValidateType(value); err != nil {
		t.Fatal(err)
	}
	top := compileText(t, katschema.NewCompiler(), `{"extra": 2}`)
	invalid, err := katschema.Overlay(value, top)
	if err != nil {
		t.Fatal(err)
	}
	if err := schema.ValidateType(invalid); !errors.Is(err, katschema.ErrValidation) {
		t.Fatalf("accepted value violating its own length constraint: %v", err)
	}
}
