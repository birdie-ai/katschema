package compiled

import (
	"testing"

	"github.com/birdie-ai/katschema/math/decimal"
)

func TestTypeCheck(t *testing.T) {
	t.Parallel()

	type testcase struct {
		name      string
		schema    string
		value     string
		check     func(*testing.T, *Arena, TypeID)
		wantError bool
	}

	for _, tc := range []testcase{
		{
			name:   "literal keeps int context",
			schema: "(int)",
			value:  "10",
			check: func(t *testing.T, a *Arena, got TypeID) {
				if got != compileText(t, a, "10") {
					t.Fatalf("typed literal = %d, want the canonical int literal", got)
				}
			},
		},
		{
			name:   "literal int schema returns literal",
			schema: "10",
			value:  "10",
			check: func(t *testing.T, a *Arena, got TypeID) {
				if got != compileText(t, a, "10") {
					t.Fatalf("typed literal = %d, want the canonical int literal", got)
				}
			},
		},
		{
			name:   "sum schema returns type of matched value - literal int",
			schema: "(int) | (string)",
			value:  "10",
			check: func(t *testing.T, a *Arena, got TypeID) {
				if got != compileText(t, a, "10") {
					t.Fatalf("typed literal = %d, want the canonical int literal", got)
				}
			},
		},
		{
			name:   "sum schema returns type of matched value - string",
			schema: "(int) | (string)",
			value:  `"test"`,
			check: func(t *testing.T, a *Arena, got TypeID) {
				if got != compileText(t, a, `"test"`) {
					t.Fatalf("typed literal = %d, want the canonical int literal", got)
				}
			},
		},
		{
			name:   "list entries are converted",
			schema: "[(int)]",
			value:  "[1, 2, 3]",
			check: func(t *testing.T, a *Arena, got TypeID) {
				if got != compileText(t, a, "[1, 2, 3]") {
					t.Fatalf("typed literal = %d, want the canonical int literal", got)
				}
			},
		},
		{
			name:   "integer is interpreted as real",
			schema: "(real)",
			value:  "10",
			check: func(t *testing.T, a *Arena, got TypeID) {
				if a.Type(got).Base().ID() != a.Real() {
					t.Fatalf("typed literal base = %d, want real %d", a.Type(got).Base().ID(), a.Real())
				}
				checkDecimal(t, a, got, "10", RealAtom)
			},
		},
		{
			name:   "decimal is interpreted as float32",
			schema: "(float32, x < 100.0)",
			value:  "31.5",
			check: func(t *testing.T, a *Arena, got TypeID) {
				if a.Type(got).Base().ID() != a.Float32() {
					t.Fatalf("typed literal base = %d, want float32 %d", a.Type(got).Base().ID(), a.Float32())
				}
				checkDecimal(t, a, got, "31.5", RealAtom)
			},
		},
		{
			name: "object fields are interpreted recursively",
			schema: `{
				"temp": (float32),
				"name": (string),
				"metadata": {"a": (string, x in ["abc", "xyz"])}
			}`,
			value: `{"metadata": {"a": "abc"}, "temp": 31.5, "name": "weather"}`,
			check: func(t *testing.T, a *Arena, got TypeID) {
				fields := a.Type(got).Fields()
				if fields.Len() != 3 {
					t.Fatalf("unexpected length: %d, expected 3", fields.Len())
				}
				temp := fields.MustGet("temp")
				name := fields.MustGet("name")
				meta := fields.MustGet("metadata")
				if tempBase := temp.Type().Base().ID(); tempBase != a.Float32() {
					t.Fatalf("temp base = %d, want float32 %d", tempBase, a.Float32())
				}
				if nameType := name.Type().Base().ID(); nameType != a.String() {
					t.Fatalf("name is not a string: %d", nameType)
				}
				metaFields := meta.Type().Fields()
				if metaFields.Len() != 1 {
					t.Fatalf("unexpected metadata fields length")
				}
				aField := metaFields.MustGet("a")
				if abase := aField.Type().Base().ID(); abase != a.String() {
					t.Fatal("unexpected metadata.a base type")
				}
				_, ok := a.Literal(aField.Type().ID())
				if !ok {
					t.Fatal("a value is not a literal")
				}
			},
		},
		{

			name: "schema with object and optional fields makes returned value omit not present fields",
			schema: `{
				"temp": (float32),
				"name": (string),
				"metadata": ({"a": (string, x in ["abc", "xyz"])}, optional)
			}`,
			value: `{"temp": 31.5, "name": "weather"}`,
			check: func(t *testing.T, a *Arena, got TypeID) {
				fields := a.Type(got).Fields()
				if fields.Len() != 2 {
					t.Fatalf("unexpected length: %d, expected 3", fields.Len())
				}
				temp := fields.MustGet("temp")
				name := fields.MustGet("name")
				if tempBase := temp.Type().Base().ID(); tempBase != a.Float32() {
					t.Fatalf("temp base = %d, want float32 %d", tempBase, a.Float32())
				}
				if nameType := name.Type().Base().ID(); nameType != a.String() {
					t.Fatalf("name is not a string: %d", nameType)
				}
			},
		},
		{
			name:      "value outside schema is rejected",
			schema:    "(int)",
			value:     "10.5",
			wantError: true,
		},
		{
			name:   "refined value keeps its context",
			schema: "(int, x >= 0)",
			value:  "(int, x > 10)",
			check: func(t *testing.T, a *Arena, got TypeID) {
				want := compileText(t, a, "(int, x > 10)")
				if got != want {
					t.Fatalf("typed refined value = %d, want original value context %d", got, want)
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := NewArena()
			schema := compileText(t, a, tc.schema)
			value := compileText(t, a, tc.value)
			got, err := a.TypeCheck(schema, value)
			if tc.wantError {
				if err == nil {
					t.Fatalf("TypeCheck(%d, %d) unexpectedly succeeded with %d", schema, value, got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !a.Subtype(got, schema) {
				t.Fatalf("typed value %d is not a subtype of schema %d", got, schema)
			}
			tc.check(t, a, got)
		})
	}
}

func checkDecimal(t *testing.T, a *Arena, got TypeID, want string, wantKind Kind) {
	atom, ok := a.Literal(got)
	if !ok || a.Node(atom).Kind() != wantKind {
		t.Fatalf("typed literal atom = %d, %t, want a real literal", atom, ok)
	}
	gotVal, ok := a.DecimalValue(atom)
	if !ok {
		t.Fatalf("not a decimal: %d", got)
	}
	if want := decimal.New(want); !gotVal.Equal(want) {
		t.Fatalf("got [%s] != want [%s]", gotVal, want)
	}
}
