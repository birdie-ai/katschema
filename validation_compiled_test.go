package katschema_test

import (
	"errors"
	"math/big"
	"testing"

	"github.com/birdie-ai/katschema"
	"github.com/birdie-ai/katschema/ks"
)

func TestValidationCompiledValues(t *testing.T) {
	t.Parallel()

	large := new(big.Int).Lsh(big.NewInt(1), 100)
	next := new(big.Int).Add(large, big.NewInt(1))
	singleton := ks.Where(ks.Int(), ks.Binary(ks.X(), ks.Eq, ks.IntExpr(1)))
	length := ks.Binary(
		ks.Binary(ks.Funcall("len", ks.X()), ks.Ge, ks.IntExpr(1)),
		ks.And,
		ks.Binary(ks.Funcall("len", ks.X()), ks.Le, ks.IntExpr(1)),
	)
	type testcase struct {
		name   string
		schema ks.Value
		value  ks.Value
		valid  bool
	}
	for _, tc := range []testcase{
		{"singleton int", ks.Int(), singleton, true},
		{"singleton under any", ks.Any(), singleton, true},
		{"nested singleton", ks.Any(), ks.Object(ks.Field("id", singleton)), true},
		{"nested schema", ks.Any(), ks.Object(ks.Field("id", ks.Int())), false},
		{"nested never", ks.Any(), ks.Object(ks.Field("id", ks.Never())), false},
		{"list schema", ks.Any(), ks.List(ks.Int()), false},
		{"optional singleton", ks.Any(), ks.Object(ks.Field("id", ks.Optional(singleton))), false},
		{"never value", ks.Any(), ks.Never(), false},
		{"never schema", ks.Never(), ks.LitNull(), false},
		{"tuple length", ks.Where(ks.List(ks.Int()), length), ks.List(ks.LitInt(1)), true},
		{"empty tuple length", ks.Where(ks.List(ks.Int()), length), ks.List(), false},
		{"object length", ks.Where(ks.Object(ks.Field("id", ks.Optional(ks.Int()))), length), ks.Object(ks.Field("id", ks.LitInt(1))), true},
		{"empty object length", ks.Where(ks.Object(ks.Field("id", ks.Optional(ks.Int()))), length), ks.Object(), false},
		{"unicode length", ks.Where(ks.String(), length), ks.LitString("é"), true},
		{"large integer", ks.LitBigInt(large), ks.LitBigInt(large), true},
		{"neighboring integer", ks.LitBigInt(large), ks.LitBigInt(next), false},
		{"exact decimal", ks.LitDecimal("9007199254740993.125"), ks.LitDecimal("9007199254740993.1250"), true},
		{"neighboring decimal", ks.LitDecimal("9007199254740993.125"), ks.LitDecimal("9007199254740993.126"), false},
		{"large exponent", ks.Real(), ks.LitDecimal("1e1000000"), true},
		{"integer as real", ks.Real(), ks.LitBigInt(large), true},
		{"decimal as integer", ks.Int(), ks.LitDecimal("1.0"), false},
		{"float singleton", ks.Real(), ks.Where(ks.Float64(), ks.Binary(ks.X(), ks.Eq, ks.DecimalExpr("0.1"))), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cc := katschema.NewCompiler()
			schema := compile(t, cc, tc.schema)
			for _, cc := range []*katschema.Compiler{cc, katschema.NewCompiler()} {
				value := compile(t, cc, tc.value)
				err := schema.ValidateType(value)
				if (err == nil) != tc.valid {
					t.Errorf("ValidateType = %v, want valid=%v", err, tc.valid)
				}
				if err != nil && !errors.Is(err, katschema.ErrValidation) {
					t.Errorf("ValidateType error does not wrap ErrValidation: %v", err)
				}
			}
			err := schema.Validate(tc.value)
			if (err == nil) != tc.valid {
				t.Errorf("Validate = %v, want valid=%v", err, tc.valid)
			}
			if err != nil && !errors.Is(err, katschema.ErrValidation) {
				t.Errorf("Validate error does not wrap ErrValidation: %v", err)
			}
		})
	}
}

func TestValidationPreservesTypes(t *testing.T) {
	t.Parallel()

	cc := katschema.NewCompiler()
	schema := compile(t, cc, ks.Object(ks.Field("number", ks.Real())))
	for _, name := range []string{"same compiler", "different compiler"} {
		if name == "different compiler" {
			cc = katschema.NewCompiler()
		}
		value := compile(t, cc, ks.Object(ks.Field("number", ks.LitInt(7))))
		schemaFingerprint, valueFingerprint := schema.Fingerprint(), value.Fingerprint()
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if err := schema.ValidateType(value); err != nil {
				t.Fatal(err)
			}
			if schema.Fingerprint() != schemaFingerprint || value.Fingerprint() != valueFingerprint {
				t.Fatal("validation changed a type fingerprint")
			}
			field, ok := value.Field("number")
			if !ok {
				t.Fatal("validation removed the value field")
			}
			number, ok := field.Type.IntValue()
			if !ok || number.Int64() != 7 {
				t.Fatalf("validation changed integer value: %v, %v", number, ok)
			}
			if _, ok := field.Type.DecimalValue(); ok {
				t.Fatal("validation converted the integer to a real")
			}
		})
	}
}

func TestValidationRejectsDuplicateFields(t *testing.T) {
	t.Parallel()

	value := ks.Object(ks.Field("id", ks.LitInt(1)), ks.Field("id", ks.LitInt(2)))
	for _, schema := range []ks.Value{ks.Any(), ks.Object(ks.Field("id", ks.Int()))} {
		typ := compile(t, katschema.NewCompiler(), schema)
		if err := typ.Validate(value); !errors.Is(err, katschema.ErrValidation) {
			t.Errorf("Validate duplicate fields = %v, want ErrValidation", err)
		}
	}
}
