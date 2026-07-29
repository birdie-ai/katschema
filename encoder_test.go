package katschema_test

import (
	"errors"
	"testing"

	ks "github.com/birdie-ai/katschema"
	"github.com/google/go-cmp/cmp"
)

type encoded struct {
	val string
	typ string
}

type encodertc struct {
	name   string
	input  ks.Value
	output encoded

	err error
}

func TestMarshaler(t *testing.T) {
	t.Parallel()

	for _, tc := range []encodertc{
		{
			name:  "null",
			input: ks.New(ks.Null, nil),
			output: encoded{
				val: `null`,
				typ: `(null)`,
			},
		},
		{
			name:  "true",
			input: ks.True,
			output: encoded{
				val: `true`,
				typ: `(bool)`,
			},
		},
		{
			name:  "false",
			input: ks.False,
			output: encoded{
				val: `false`,
				typ: `(bool)`,
			},
		},
		{
			name:  "int",
			input: ks.New(ks.Int, 10),
			output: encoded{
				val: `10`,
				typ: `(int)`,
			},
		},
		{
			name:  "float integer part",
			input: ks.New(ks.Float, 10),
			output: encoded{
				val: `10`,
				typ: `(float)`,
			},
		},
		{
			name:  "float decimal part",
			input: ks.New(ks.Float, 10.5),
			output: encoded{
				val: `10.5`,
				typ: `(float)`,
			},
		},
		{
			name:  "string",
			input: ks.New(ks.String, "test"),
			output: encoded{
				val: `"test"`,
				typ: `(string)`,
			},
		},
		{
			name:  "list of string using Go types",
			input: ks.New(ks.List{Items: ks.String}, []string{"a", "b"}),
			output: encoded{
				val: `["a","b"]`,
				typ: `[(string)]`,
			},
		},
		{
			name: "list of string using AST values",
			input: ks.New(ks.List{Items: ks.String}, []ks.Value{
				{
					Type:  ks.String,
					Value: "a",
				},
				{
					Type:  ks.String,
					Value: "b",
				},
			}),
			output: encoded{
				val: `["a","b"]`,
				typ: `[(string)]`,
			},
		},
		{
			name:  "list of int",
			input: ks.New(ks.List{Items: ks.Int}, []int{0, 1, 2}),
			output: encoded{
				val: `[0,1,2]`,
				typ: `[(int)]`,
			},
		},
		{
			name:  "list of bool",
			input: ks.New(ks.List{Items: ks.Bool}, []bool{false, false, true, false}),
			output: encoded{
				val: `[false,false,true,false]`,
				typ: `[(bool)]`,
			},
		},
		{
			name:  "empty object value",
			input: ks.New(ks.Object[string]{}, []ks.Value{}),
			output: encoded{
				val: `{}`,
				typ: `{}`,
			},
		},
		{
			name: "object string->value",
			input: ks.New(ks.Object[string]{
				Fields: map[string]ks.Schema{
					"a": ks.String,
					"b": ks.Int,
				},
			}, []ks.Value{
				ks.New(ks.Tuple{ks.String, ks.String}, []ks.Value{
					ks.New(ks.String, "a"),
					ks.New(ks.String, "some value"),
				}),
				ks.New(ks.Tuple{ks.String, ks.String}, []ks.Value{
					ks.New(ks.String, "b"),
					ks.New(ks.Int, 10),
				}),
			}),
			output: encoded{
				val: `{"a":"some value","b":10}`,
				typ: `{"a":(string),"b":(int)}`,
			},
		},
		{
			name: "object int fields -> value",
			input: ks.New(ks.Object[int]{
				Fields: map[int]ks.Schema{
					1: ks.String,
					2: ks.Int,
				},
			}, []ks.Value{
				ks.New(ks.Tuple{ks.Int, ks.String}, []ks.Value{
					ks.New(ks.Int, 1),
					ks.New(ks.String, "some value"),
				}),
				ks.New(ks.Tuple{ks.Int, ks.String}, []ks.Value{
					ks.New(ks.Int, 2),
					ks.New(ks.Int, 10),
				}),
			}),
			output: encoded{
				val: `{1:"some value",2:10}`,
				typ: `{1:(string),2:(int)}`,
			},
		},
		{
			name: "object value from ref type",
			input: ks.New(ks.Schema{
				Ref: ks.Ref("sometype"),
				Impl: ks.Object[string]{
					Fields: map[string]ks.Schema{
						"a": ks.String,
						"b": ks.Int,
					},
				},
			}, []ks.Value{
				ks.New(ks.Tuple{ks.String, ks.String}, []ks.Value{
					ks.New(ks.String, "a"),
					ks.New(ks.String, "some value"),
				}),
				ks.New(ks.Tuple{ks.String, ks.String}, []ks.Value{
					ks.New(ks.String, "b"),
					ks.New(ks.Int, 10),
				}),
			}),
			output: encoded{
				val: `{"a":"some value","b":10}`,
				typ: `(sometype)`,
			},
		},
	} {
		t.Run(tc.name+"/value", func(t *testing.T) {
			got, err := tc.input.MarshalText()
			if !errors.Is(err, tc.err) {
				t.Fatal(err)
			}
			if err != nil {
				return
			}
			assert(t, tc.output.val, string(got))
		})
		t.Run(tc.name+"/type", func(t *testing.T) {
			got, err := tc.input.Type.MarshalText()
			if !errors.Is(err, tc.err) {
				t.Fatal(err)
			}
			if err != nil {
				return
			}
			assert(t, tc.output.typ, string(got))
		})
	}
}

func assert(t *testing.T, want, got any) {
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatal(diff)
	}
}
