package katschema_test

import (
	"math"
	"math/big"
	"strconv"
	"testing"

	"github.com/birdie-ai/katschema"
	"github.com/birdie-ai/katschema/ks"
	"github.com/birdie-ai/katschema/math/decimal"
)

type scalarTestcase struct {
	name     string
	value    ks.Value
	kind     katschema.Kind
	want     string
	wantJSON string
}

func scalarTestcases() []scalarTestcase {
	large := new(big.Int).Lsh(big.NewInt(1), 100)
	return []scalarTestcase{
		{"null", ks.LitNull(), katschema.Null, "", "null"},
		{"false", ks.LitBool(false), katschema.Bool, "false", "false"},
		{"true", ks.LitBool(true), katschema.Bool, "true", "true"},
		{"empty string", ks.LitString(""), katschema.String, "", `""`},
		{"string", ks.LitString("birdie"), katschema.String, "birdie", `"birdie"`},
		{"escaped string", ks.LitString("Olá\n\"birdie\""), katschema.String, "Olá\n\"birdie\"", `"Olá\n\"birdie\""`},
		{"zero", ks.LitInt(0), katschema.Int, "0", "0"},
		{"min int64", ks.LitInt(math.MinInt64), katschema.Int, "-9223372036854775808", "-9223372036854775808"},
		{"max int64", ks.LitInt(math.MaxInt64), katschema.Int, "9223372036854775807", "9223372036854775807"},
		{"large integer", ks.LitBigInt(large), katschema.Int, large.String(), large.String()},
		{"negative large integer", ks.LitBigInt(new(big.Int).Neg(large)), katschema.Int, "-" + large.String(), "-" + large.String()},
		{"decimal", ks.LitDecimal("-123.4500"), katschema.Real, "-12345e-2", "-12345e-2"},
		{"decimal zero", ks.LitDecimal("-0.00"), katschema.Real, "0", "0e0"},
		{"integral decimal", ks.LitDecimal("1.0"), katschema.Real, "1", "1e0"},
		{"scaled integral decimal", ks.LitDecimal("100.0"), katschema.Real, "1e2", "1e2"},
		{"precise decimal", ks.LitDecimal("9007199254740993.125"), katschema.Real, "9007199254740993125e-3", "9007199254740993125e-3"},
		{"large exponent", ks.LitDecimal("1e1000000"), katschema.Real, "1e1000000", "1e1000000"},
		{"small exponent", ks.LitDecimal("1e-1000000"), katschema.Real, "1e-1000000", "1e-1000000"},
		{"singleton int", ks.With(ks.Int(),
			ks.Check(ks.Binary(ks.X(), ks.Ge, ks.IntExpr(10))),
			ks.Check(ks.Binary(ks.X(), ks.Le, ks.IntExpr(10))),
		), katschema.Int, "10", "10"},
		{"singleton equality", ks.Where(ks.Int(), ks.Binary(ks.X(), ks.Eq, ks.IntExpr(7))), katschema.Int, "7", "7"},
		{"singleton string", ks.With(ks.String(),
			ks.Check(ks.Binary(ks.X(), ks.Eq, ks.StrExpr("one"))),
		), katschema.String, "one", `"one"`},
		{"metadata", ks.With(ks.Int(), ks.Check(ks.Binary(ks.X(), ks.Eq, ks.IntExpr(7))), ks.Flag("generated")), katschema.Int, "7", "7"},
	}
}

func TestScalarValues(t *testing.T) {
	t.Parallel()

	for _, tc := range scalarTestcases() {
		t.Run(tc.name, func(t *testing.T) {
			typ := compile(t, katschema.NewCompiler(), tc.value)
			checkScalar(t, typ, tc.kind, tc.want)
		})
	}
}

func TestScalarValuesRejectsNonScalars(t *testing.T) {
	t.Parallel()

	type testcase struct {
		name  string
		value ks.Value
	}
	for _, tc := range []testcase{
		{"any", ks.Any()},
		{"never", ks.Never()},
		{"bool type", ks.Bool()},
		{"int type", ks.Int()},
		{"real type", ks.Real()},
		{"string type", ks.String()},
		{"range", ks.With(ks.Int(), ks.Check(ks.Binary(ks.X(), ks.Gt, ks.IntExpr(0))))},
		{"sum", ks.Sum(ks.LitInt(1), ks.LitInt(2))},
		{"object", ks.Object(ks.Field("id", ks.LitInt(1)))},
		{"tuple", ks.List(ks.LitInt(1))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			typ := compile(t, katschema.NewCompiler(), tc.value)
			checkScalar(t, typ, katschema.Invalid, "")
		})
	}
	checkScalar(t, katschema.Type{}, katschema.Invalid, "")
}

func checkScalar(t *testing.T, typ katschema.Type, kind katschema.Kind, want string) {
	t.Helper()

	if got := typ.IsNull(); got != (kind == katschema.Null) {
		t.Errorf("IsNull = %v, want %v", got, kind == katschema.Null)
	}
	b, isBool := typ.BoolValue()
	if isBool != (kind == katschema.Bool) {
		t.Errorf("BoolValue ok = %v, want %v", isBool, kind == katschema.Bool)
	}
	s, isString := typ.StringValue()
	if isString != (kind == katschema.String) {
		t.Errorf("StringValue ok = %v, want %v", isString, kind == katschema.String)
	}
	i, isInt := typ.IntValue()
	if isInt != (kind == katschema.Int) {
		t.Errorf("IntValue ok = %v, want %v", isInt, kind == katschema.Int)
	}
	d, isReal := typ.DecimalValue()
	if isReal != (kind == katschema.Real) {
		t.Errorf("DecimalValue ok = %v, want %v", isReal, kind == katschema.Real)
	}

	switch kind {
	case katschema.Null, katschema.Invalid:
		return
	case katschema.Bool:
		if isBool && strconv.FormatBool(b) != want {
			t.Errorf("BoolValue = %v, want %q", b, want)
		}
	case katschema.String:
		if isString && s != want {
			t.Errorf("StringValue = %q, want %q", s, want)
		}
	case katschema.Int:
		if isInt && i.String() != want {
			t.Errorf("IntValue = %v, want %q", i, want)
		}
	case katschema.Real:
		if isReal && !d.Equal(decimal.New(want)) {
			t.Errorf("DecimalValue = %v, want %q", d, want)
		}
	default:
		t.Fatal("unhandled scalar kind:", kind)
	}
}

func TestScalarValuesOwnTheirData(t *testing.T) {
	t.Parallel()

	for _, n := range []*big.Int{big.NewInt(7), new(big.Int).Lsh(big.NewInt(1), 100)} {
		typ := compile(t, katschema.NewCompiler(), ks.LitBigInt(n))
		got, ok := typ.IntValue()
		if !ok {
			t.Fatal("IntValue returned no value")
		}
		got.SetInt64(0)
		checkScalar(t, typ, katschema.Int, n.String())
	}
	typ := compile(t, katschema.NewCompiler(), ks.LitDecimal("1.25"))
	got, ok := typ.DecimalValue()
	if !ok {
		t.Fatal("DecimalValue returned no value")
	}
	got.Digits[0] = '9'
	checkScalar(t, typ, katschema.Real, "1.25")
}

func TestScalarValuesThroughOverlay(t *testing.T) {
	t.Parallel()

	bottom := compile(t, katschema.NewCompiler(), ks.Object(
		ks.Field("name", ks.LitString("birdie")),
		ks.Field("version", ks.LitInt(1)),
	))
	top := compile(t, katschema.NewCompiler(), ks.Object(
		ks.Field("version", ks.LitInt(2)),
	))
	merged, err := katschema.Overlay(bottom, top)
	if err != nil {
		t.Fatal(err)
	}
	checkScalar(t, merged, katschema.Invalid, "")
	name, ok := merged.Field("name")
	if !ok {
		t.Fatal("missing name")
	}
	checkScalar(t, name.Type, katschema.String, "birdie")
	version, ok := merged.Field("version")
	if !ok {
		t.Fatal("missing version")
	}
	checkScalar(t, version.Type, katschema.Int, "2")
}
