package katschema_test

import (
	"encoding/json"
	"testing"

	"github.com/birdie-ai/katschema"
	"github.com/birdie-ai/katschema/ks"
)

func TestMarshalJSON(t *testing.T) {
	t.Parallel()

	type testcase struct {
		name  string
		value ks.Value
		want  string
	}
	tests := []testcase{
		{"empty array", ks.List(), "[]"},
		{"empty object", ks.Object(), "{}"},
		{"nested arrays", ks.List(ks.LitInt(3), ks.LitNull(), ks.LitInt(3), ks.List(ks.LitString("x"), ks.LitBool(false))), `[3,null,3,["x",false]]`},
		{"object fields", ks.Object(ks.Field("z", ks.List()), ks.Field("a", ks.Object())), `{"a":{},"z":[]}`},
	}
	for _, tc := range scalarTestcases() {
		tests = append(tests, testcase{tc.name, tc.value, tc.wantJSON})
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			value := compile(t, katschema.NewCompiler(), tc.value)
			got, err := value.MarshalJSON()
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.want {
				t.Fatalf("MarshalJSON = %s, want %s", got, tc.want)
			}
			if !json.Valid(got) {
				t.Fatalf("MarshalJSON returned invalid JSON: %q", got)
			}
			wrapped, err := json.Marshal(value)
			if err != nil || string(wrapped) != tc.want {
				t.Fatalf("json.Marshal = %s, %v; want %s", wrapped, err, tc.want)
			}
		})
	}
}

func TestMarshalJSONStrings(t *testing.T) {
	t.Parallel()

	controls := make([]byte, 32)
	for i := range controls {
		controls[i] = byte(i)
	}
	for _, text := range []string{string(controls), "quote\"\\/", "Olá 🐦", "\u2028\u2029", "<birdie>&"} {
		t.Run(text, func(t *testing.T) {
			value := compile(t, katschema.NewCompiler(), ks.Object(ks.Field(text, ks.LitString(text))))
			got, err := value.MarshalJSON()
			if err != nil {
				t.Fatal(err)
			}
			var decoded map[string]string
			if err := json.Unmarshal(got, &decoded); err != nil {
				t.Fatalf("invalid JSON %q: %v", got, err)
			}
			if actual, ok := decoded[text]; len(decoded) != 1 || !ok || actual != text {
				t.Fatalf("decoded JSON = %q, want key and value %q", decoded, text)
			}
		})
	}
}

func TestMarshalJSONOverlay(t *testing.T) {
	t.Parallel()

	bottom := compile(t, katschema.NewCompiler(), ks.With(ks.Object(
		ks.Field("id", ks.With(ks.String(), ks.Check(ks.Binary(ks.X(), ks.Eq, ks.StrExpr("one"))), ks.Flag("pk"))),
		ks.Field("version", ks.LitInt(1)),
	), ks.Attr("source", ks.StrExpr("input"))))
	top := compile(t, katschema.NewCompiler(), ks.Object(
		ks.Field("version", ks.LitInt(2)),
		ks.Field("updated_at", ks.With(ks.Int(), ks.Check(ks.Binary(ks.X(), ks.Eq, ks.IntExpr(10))), ks.Flag("generated"))),
	))
	value, err := katschema.Overlay(bottom, top)
	if err != nil {
		t.Fatal(err)
	}
	got, err := value.MarshalJSON()
	const want = `{"id":"one","updated_at":10,"version":2}`
	if err != nil || string(got) != want {
		t.Fatalf("MarshalJSON overlay = %s, %v; want %s", got, err, want)
	}
	got, err = bottom.MarshalJSON()
	if err != nil || string(got) != `{"id":"one","version":1}` {
		t.Fatalf("MarshalJSON changed original value: %s, %v", got, err)
	}
}

func TestMarshalJSONRejectsInvalidValues(t *testing.T) {
	t.Parallel()

	singleton := ks.Where(ks.Int(), ks.Binary(ks.X(), ks.Eq, ks.IntExpr(1)))
	type testcase struct {
		name  string
		value ks.Value
	}
	for _, tc := range []testcase{
		{"any", ks.Any()},
		{"never", ks.Never()},
		{"int", ks.Int()},
		{"sum", ks.Sum(ks.LitInt(1), ks.LitInt(2))},
		{"range", ks.Where(ks.Int(), ks.Binary(ks.X(), ks.Gt, ks.IntExpr(0)))},
		{"list schema", ks.List(ks.Int())},
		{"object schema", ks.Object(ks.Field("id", ks.Int()))},
		{"optional singleton", ks.Object(ks.Field("id", ks.Optional(singleton)))},
		{"nested schema", ks.Object(ks.Field("nested", ks.Object(ks.Field("id", ks.Int()))))},
		{"invalid string", ks.LitString("\xff")},
		{"invalid field name", ks.Object(ks.Field("\xff", ks.LitInt(1)))},
		{"nested invalid string", ks.List(ks.LitInt(1), ks.LitString("\xff"))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			value := compile(t, katschema.NewCompiler(), tc.value)
			got, err := value.MarshalJSON()
			if err == nil || got != nil {
				t.Fatalf("MarshalJSON = %q, %v; want nil bytes and error", got, err)
			}
			if _, err := json.Marshal(value); err == nil {
				t.Fatal("json.Marshal accepted invalid value")
			}
		})
	}
	if got, err := (katschema.Type{}).MarshalJSON(); err == nil || got != nil {
		t.Fatalf("zero Type MarshalJSON = %q, %v; want nil bytes and error", got, err)
	}
}

func TestMarshalJSONRejectsInvalidOverlay(t *testing.T) {
	t.Parallel()

	bottom := compile(t, katschema.NewCompiler(), ks.Where(
		ks.Object(ks.Field("id", ks.LitInt(1))),
		ks.Binary(ks.Funcall("len", ks.X()), ks.Le, ks.IntExpr(1)),
	))
	top := compile(t, katschema.NewCompiler(), ks.Object(ks.Field("extra", ks.LitInt(2))))
	value, err := katschema.Overlay(bottom, top)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := value.MarshalJSON(); err == nil || got != nil {
		t.Fatalf("MarshalJSON invalid overlay = %q, %v; want nil bytes and error", got, err)
	}
}
