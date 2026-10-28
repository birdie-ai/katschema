package katschema_test

import (
	"errors"
	"testing"

	"github.com/birdie-ai/katschema"
	"github.com/birdie-ai/katschema/ks"
)

func TestValidateType(t *testing.T) {
	t.Parallel()

	nonempty := ks.Where(ks.String(), ks.Binary(ks.Funcall("len", ks.X()), ks.Gt, ks.IntExpr(0)))
	tags := ks.Where(ks.List(ks.String()), ks.Binary(ks.Funcall("len", ks.X()), ks.Le, ks.IntExpr(2)))
	schema := compile(t, katschema.NewCompiler(), ks.Object(
		ks.Field("id", nonempty),
		ks.Field("tags", ks.Optional(tags)),
	))
	type testcase struct {
		name  string
		value ks.Value
		valid bool
	}
	for _, tc := range []testcase{
		{"optional omitted", ks.Object(ks.Field("id", ks.LitString("one"))), true},
		{"empty array", ks.Object(ks.Field("id", ks.LitString("one")), ks.Field("tags", ks.List())), true},
		{"array at limit", ks.Object(ks.Field("id", ks.LitString("one")), ks.Field("tags", ks.List(ks.LitString("a"), ks.LitString("b")))), true},
		{"array over limit", ks.Object(ks.Field("id", ks.LitString("one")), ks.Field("tags", ks.List(ks.LitString("a"), ks.LitString("b"), ks.LitString("c")))), false},
		{"wrong element", ks.Object(ks.Field("id", ks.LitString("one")), ks.Field("tags", ks.List(ks.LitInt(1)))), false},
		{"empty id", ks.Object(ks.Field("id", ks.LitString(""))), false},
		{"missing id", ks.Object(), false},
		{"unknown field", ks.Object(ks.Field("id", ks.LitString("one")), ks.Field("internal", ks.LitInt(1))), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			value := compile(t, katschema.NewCompiler(), tc.value)
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
	anyType := compile(t, cc, ks.Any())
	type testcase struct {
		name  string
		value katschema.Type
	}
	for _, tc := range []testcase{
		{"zero", katschema.Type{}},
		{"any", anyType},
		{"never", compile(t, cc, ks.Never())},
		{"int", compile(t, cc, ks.Int())},
		{"range", compile(t, cc, ks.Where(ks.Int(), ks.Binary(ks.X(), ks.Gt, ks.IntExpr(0))))},
		{"sum", compile(t, cc, ks.Sum(ks.LitInt(1), ks.LitInt(2)))},
		{"list schema", compile(t, cc, ks.List(ks.Int()))},
		{"object schema", compile(t, cc, ks.Object(ks.Field("id", ks.String())))},
		{"optional field", compile(t, cc, ks.Object(ks.Field("id", ks.Optional(ks.Where(ks.Int(), ks.Binary(ks.X(), ks.Eq, ks.IntExpr(1)))))))},
		{"nested schema", compile(t, cc, ks.Object(ks.Field("data", ks.Object(ks.Field("value", ks.Any())))))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := anyType.ValidateType(tc.value); !errors.Is(err, katschema.ErrValidation) {
				t.Fatalf("ValidateType = %v, want ErrValidation", err)
			}
		})
	}
	value := compile(t, cc, ks.LitInt(1))
	if err := (katschema.Type{}).ValidateType(value); !errors.Is(err, katschema.ErrValidation) {
		t.Fatalf("zero schema ValidateType = %v, want ErrValidation", err)
	}
}

func TestValidateTypeOverlays(t *testing.T) {
	t.Parallel()

	input := compile(t, katschema.NewCompiler(), ks.With(ks.Object(
		ks.Field("id", ks.String()),
	), ks.Flag("entity")))
	internal := compile(t, katschema.NewCompiler(), ks.Object(ks.Field("updated_at", ks.Int())))
	schema, err := katschema.Overlay(input, internal)
	if err != nil {
		t.Fatal(err)
	}
	public := compile(t, katschema.NewCompiler(), ks.With(ks.Object(
		ks.Field("id", ks.LitString("one")),
	), ks.Flag("source")))
	generated := compile(t, katschema.NewCompiler(), ks.Object(ks.Field("updated_at", ks.LitInt(10))))
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
	schema := compile(t, cc, ks.Any())
	value := compile(t, cc, ks.Where(ks.Object(ks.Field("id", ks.LitInt(1))),
		ks.Binary(ks.Funcall("len", ks.X()), ks.Le, ks.IntExpr(1)),
	))
	if err := schema.ValidateType(value); err != nil {
		t.Fatal(err)
	}
	top := compile(t, katschema.NewCompiler(), ks.Object(ks.Field("extra", ks.LitInt(2))))
	invalid, err := katschema.Overlay(value, top)
	if err != nil {
		t.Fatal(err)
	}
	if err := schema.ValidateType(invalid); !errors.Is(err, katschema.ErrValidation) {
		t.Fatalf("accepted value violating its own length constraint: %v", err)
	}
}
