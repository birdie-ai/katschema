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
		value string
		want  string
	}
	for _, tc := range []testcase{
		{name: "empty array", value: "[]", want: "[]"},
		{name: "empty object", value: "{}", want: "{}"},
		{name: "nested arrays", value: `[3, null, 3, ["x", false]]`, want: `[3,null,3,["x",false]]`},
		{name: "object fields", value: `{"z": [], "a": {}}`, want: `{"a":{},"z":[]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			value := compileText(t, katschema.NewCompiler(), tc.value)
			checkMarshalJSON(t, value, tc.want)
		})
	}
	for _, tc := range scalarTestcases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			value := compile(t, katschema.NewCompiler(), tc.value)
			checkMarshalJSON(t, value, tc.wantJSON)
		})
	}
}

func checkMarshalJSON(t *testing.T, value katschema.Type, want string) {
	t.Helper()
	got, err := value.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("MarshalJSON = %s, want %s", got, want)
	}
	if !json.Valid(got) {
		t.Fatalf("MarshalJSON returned invalid JSON: %q", got)
	}
	wrapped, err := json.Marshal(value)
	if err != nil || string(wrapped) != want {
		t.Fatalf("json.Marshal = %s, %v; want %s", wrapped, err, want)
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

	bottom := compileText(t, katschema.NewCompiler(), `({
		"id": (string, x == "one", pk),
		"version": 1
	}, source="input")`)
	top := compileText(t, katschema.NewCompiler(), `{
		"version": 2,
		"updated_at": (int, x == 10, generated)
	}`)
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

	type testcase struct {
		name  string
		value string
	}
	for _, tc := range []testcase{
		{name: "any", value: "(any)"},
		{name: "never", value: "(never)"},
		{name: "int", value: "(int)"},
		{name: "range", value: "(int, x > 0)"},
		{name: "list schema", value: "[(int)]"},
		{name: "object schema", value: `{"id": (int)}`},
		{name: "optional singleton", value: `{"id": (int, x == 1, optional)}`},
		{name: "nested schema", value: `{"nested": {"id": (int)}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			value := compileText(t, katschema.NewCompiler(), tc.value)
			checkMarshalJSONError(t, value)
		})
	}
	if got, err := (katschema.Type{}).MarshalJSON(); err == nil || got != nil {
		t.Fatalf("zero Type MarshalJSON = %q, %v; want nil bytes and error", got, err)
	}
}

func TestMarshalJSONRejectsInvalidDSLValues(t *testing.T) {
	t.Parallel()

	type testcase struct {
		name  string
		value ks.Value
	}
	for _, tc := range []testcase{
		{name: "sum", value: ks.Sum(ks.LitInt(1), ks.LitInt(2))},
		{name: "invalid string", value: ks.LitString("\xff")},
		{name: "invalid field name", value: ks.Object(ks.Field("\xff", ks.LitInt(1)))},
		{name: "nested invalid string", value: ks.List(ks.LitInt(1), ks.LitString("\xff"))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			value := compile(t, katschema.NewCompiler(), tc.value)
			checkMarshalJSONError(t, value)
		})
	}
}

func checkMarshalJSONError(t *testing.T, value katschema.Type) {
	t.Helper()
	got, err := value.MarshalJSON()
	if err == nil || got != nil {
		t.Fatalf("MarshalJSON = %q, %v; want nil bytes and error", got, err)
	}
	if _, err := json.Marshal(value); err == nil {
		t.Fatal("json.Marshal accepted invalid value")
	}
}

func TestMarshalJSONRejectsInvalidOverlay(t *testing.T) {
	t.Parallel()

	bottom := compileText(t, katschema.NewCompiler(), `({"id": 1}, len(x) <= 1)`)
	top := compileText(t, katschema.NewCompiler(), `{"extra": 2}`)
	value, err := katschema.Overlay(bottom, top)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := value.MarshalJSON(); err == nil || got != nil {
		t.Fatalf("MarshalJSON invalid overlay = %q, %v; want nil bytes and error", got, err)
	}
}
