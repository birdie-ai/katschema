package katschema_test

import (
	"encoding"
	"errors"
	"testing"

	ks "github.com/birdie-ai/katschema"
	"github.com/google/go-cmp/cmp"
)

type encodertc struct {
	name string
	v    encoding.TextMarshaler
	w    string
	e    error
}

func TestMarshaler(t *testing.T) {
	t.Parallel()

	for _, tc := range []encodertc{
		{
			name: "null",
			v:    ks.Null{},
			w:    `null`,
		},
		{
			name: "true",
			v:    ks.True,
			w:    `true`,
		},
		{
			name: "false",
			v:    ks.False,
			w:    `false`,
		},
		{
			name: "int",
			v:    ks.New(ks.Int, 10),
			w:    `10`,
		},
		{
			name: "float integer part",
			v:    ks.New(ks.Float, 10),
			w:    `10`,
		},
		{
			name: "float decimal part",
			v:    ks.New(ks.Float, 10.5),
			w:    `10.5`,
		},
		{
			name: "string",
			v:    ks.New(ks.String, "test"),
			w:    `"test"`,
		},
		{
			name: "list",
			v:    ks.New2(ks.Schema{Type: ks.List{Items: ks.String}}, ks.Primitive{Value: []string{"a", "b"}}),
			w:    `"test"`,
		},
		{
			name: "(bool)",
			v:    ks.Bool,
			w:    `(bool)`,
		},

		{
			name: "empty object",
			v:    ks.Object[string]{},
			w:    `{}`,
		},
		{
			name: "flat object",
			v: ks.Object[string]{
				Fields: map[string]ks.Type{
					"a": ks.Object[string]{},
					"b": ks.Null{},
				},
			},
			w: `{"a":{},"b":null}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.v.MarshalText()
			if !errors.Is(err, tc.e) {
				t.Fatal(err)
			}
			if err != nil {
				return
			}
			assert(t, tc.w, string(got))
		})
	}
}

func assert(t *testing.T, want, got any) {
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatal(diff)
	}
}
