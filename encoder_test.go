package katschema_test

import (
	"errors"
	"testing"

	ks "github.com/birdie-ai/katschema"
	"github.com/google/go-cmp/cmp"
)

func TestMarshaler(t *testing.T) {
	t.Parallel()

	type encoded struct {
		val string
		typ string
	}

	type testcase struct {
		name   string
		input  ks.Value
		output encoded
		err    error
	}

	book := ks.Object[string]{
		Fields: ks.Fields[string]{
			{Key: "id", Typ: ks.String},
			{Key: "title", Typ: ks.String},
			{Key: "author", Typ: ks.String},
		},
	}

	books := ks.List{Items: book}

	user := ks.Object[string]{
		Fields: ks.Fields[string]{
			{Key: "login", Typ: ks.String},
			{Key: "books", Typ: books},
		},
	}

	for _, tc := range []testcase{
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
			name: "simple object set  using Go map - lose order",
			input: ks.New(book, map[string]any{
				"id":     "some-id",
				"title":  "Principia Mathematica",
				"author": "Isaac Newton",
			}),
			output: encoded{
				val: `{"author":"Isaac Newton","id":"some-id","title":"Principia Mathematica"}`,
				typ: `{"id":(string),"title":(string),"author":(string)}`,
			},
		},
		{
			name: "array of objects",
			input: ks.New(books, []ks.Value{
				ks.New(book, map[string]any{
					"id":     "some-other",
					"title":  "Elements",
					"author": "Euclid",
				}),
				ks.New(book, map[string]any{
					"id":     "some-id",
					"title":  "Principia Mathematica",
					"author": "Isaac Newton",
				}),
			},
			),
			output: encoded{
				val: `[{"author":"Euclid","id":"some-other","title":"Elements"},{"author":"Isaac Newton","id":"some-id","title":"Principia Mathematica"}]`,
				typ: `[{"id":(string),"title":(string),"author":(string)}]`,
			},
		},
		{
			// below is the lower-level AST that's generated when it's
			// decoded using the parser (rather than manually constructed).
			name: "simple object set using tuples - keep order",
			input: ks.New(book, []ks.Value{
				ks.New(ks.Tuple{ks.String, ks.String}, []ks.Value{
					ks.New(ks.String, "id"),
					ks.New(ks.String, "some-id"),
				}),
				ks.New(ks.Tuple{ks.String, ks.String}, []ks.Value{
					ks.New(ks.String, "title"),
					ks.New(ks.String, "Principia Mathematica"),
				}),
				ks.New(ks.Tuple{ks.String, ks.String}, []ks.Value{
					ks.New(ks.String, "author"),
					ks.New(ks.String, "Isaac Newton"),
				}),
			}),
			output: encoded{
				val: `{"id":"some-id","title":"Principia Mathematica","author":"Isaac Newton"}`,
				typ: `{"id":(string),"title":(string),"author":(string)}`,
			},
		},
		{
			name: "object string->value",
			input: ks.New(ks.Object[string]{
				Fields: ks.Fields[string]{
					{Key: "a", Typ: ks.String},
					{Key: "b", Typ: ks.Int},
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
				Fields: ks.Fields[int]{
					{Key: 1, Typ: ks.String},
					{Key: 2, Typ: ks.Int},
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
					Fields: ks.Fields[string]{
						{Key: "a", Typ: ks.String},
						{Key: "a", Typ: ks.Int},
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
		{
			name: "nested object",
			input: ks.New(user, map[string]any{
				"login": "i4k",
				"books": ks.New(books, []ks.Value{
					ks.New(book, map[string]any{
						"id":     "some-id",
						"title":  "The Winter King",
						"author": "Bernard Cornwell",
					}),
				}),
			}),
			output: encoded{
				val: `{"books":[{"author":"Bernard Cornwell","id":"some-id","title":"The Winter King"}],"login":"i4k"}`,
				typ: `{"login":(string),"books":[{"id":(string),"title":(string),"author":(string)}]}`,
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
