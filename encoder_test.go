package katschema_test

import (
	"bytes"
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

	book := ks.NewObj(
		ks.Field{Key: "id", Typ: ks.String},
		ks.Field{Key: "title", Typ: ks.String},
		ks.Field{Key: "author", Typ: ks.String},
	)

	books := ks.NewList(book)

	address := ks.NewObj(
		ks.Field{Key: "street", Typ: ks.String},
		ks.Field{Key: "number", Typ: ks.Int},
		ks.Field{Key: "state", Typ: ks.String},
		ks.Field{Key: "country", Typ: ks.String},
	)

	user := ks.NewObj(
		ks.Field{Key: "login", Typ: ks.String},
		ks.Field{Key: "address", Typ: address},
		ks.Field{Key: "books", Typ: books},
	)

	for _, tc := range []testcase{
		{
			name:  "null",
			input: ks.NewValue(ks.Null, nil),
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
			input: ks.NewValue(ks.Int, 10),
			output: encoded{
				val: `10`,
				typ: `(int)`,
			},
		},
		{
			name:  "float integer part",
			input: ks.NewValue(ks.Float, 10),
			output: encoded{
				val: `10`,
				typ: `(float)`,
			},
		},
		{
			name:  "float decimal part",
			input: ks.NewValue(ks.Float, 10.5),
			output: encoded{
				val: `10.5`,
				typ: `(float)`,
			},
		},
		{
			name:  "string",
			input: ks.NewValue(ks.String, "test"),
			output: encoded{
				val: `"test"`,
				typ: `(string)`,
			},
		},
		{
			name:  "list of string using Go types",
			input: ks.NewValue(ks.NewList(ks.String), []string{"a", "b"}),
			output: encoded{
				val: `["a","b"]`,
				typ: `[(string)]`,
			},
		},
		{
			name: "list of string using AST values",
			input: ks.NewValue(ks.NewList(ks.String), []ks.Value{
				ks.NewValue(ks.String, "a"),
				ks.NewValue(ks.String, "b"),
			}),
			output: encoded{
				val: `["a","b"]`,
				typ: `[(string)]`,
			},
		},
		{
			name:  "list of int",
			input: ks.NewValue(ks.NewList(ks.Int), []int{0, 1, 2}),
			output: encoded{
				val: `[0,1,2]`,
				typ: `[(int)]`,
			},
		},
		{
			name:  "list of bool",
			input: ks.NewValue(ks.NewList(ks.Bool), []bool{false, false, true, false}),
			output: encoded{
				val: `[false,false,true,false]`,
				typ: `[(bool)]`,
			},
		},
		{
			name:  "empty object value",
			input: ks.NewValue(ks.NewObj(), []ks.Value{}),
			output: encoded{
				val: `{}`,
				typ: `{}`,
			},
		},
		{
			name: "simple object set  using Go map - lose order",
			input: ks.NewValue(book, map[string]any{
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
			input: ks.NewValue(books, []ks.Value{
				ks.NewValue(book, map[string]any{
					"id":     "some-other",
					"title":  "Elements",
					"author": "Euclid",
				}),
				ks.NewValue(book, map[string]any{
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
			input: ks.NewValue(book, []ks.Value{
				ks.NewValue(ks.NewTuple(ks.String, ks.String), []ks.Value{
					ks.NewValue(ks.String, "id"),
					ks.NewValue(ks.String, "some-id"),
				}),
				ks.NewValue(ks.NewTuple(ks.String, ks.String), []ks.Value{
					ks.NewValue(ks.String, "title"),
					ks.NewValue(ks.String, "Principia Mathematica"),
				}),
				ks.NewValue(ks.NewTuple(ks.String, ks.String), []ks.Value{
					ks.NewValue(ks.String, "author"),
					ks.NewValue(ks.String, "Isaac Newton"),
				}),
			}),
			output: encoded{
				val: `{"id":"some-id","title":"Principia Mathematica","author":"Isaac Newton"}`,
				typ: `{"id":(string),"title":(string),"author":(string)}`,
			},
		},
		{
			name: "object string->value",
			input: ks.NewValue(ks.NewObj(
				ks.Field{Key: "a", Typ: ks.String},
				ks.Field{Key: "b", Typ: ks.Int},
			), []ks.Value{
				ks.NewValue(ks.NewTuple(ks.String, ks.String), []ks.Value{
					ks.NewValue(ks.String, "a"),
					ks.NewValue(ks.String, "some value"),
				}),
				ks.NewValue(ks.NewTuple(ks.String, ks.String), []ks.Value{
					ks.NewValue(ks.String, "b"),
					ks.NewValue(ks.Int, 10),
				}),
			}),
			output: encoded{
				val: `{"a":"some value","b":10}`,
				typ: `{"a":(string),"b":(int)}`,
			},
		},
		{
			name: "object value from ref type",
			input: ks.NewValue(ks.NewType(ks.Schema{
				Ref: ks.Ref("sometype"),
				Impl: ks.NewObj(
					ks.Field{Key: "a", Typ: ks.String},
					ks.Field{Key: "a", Typ: ks.Int},
				),
			}), []ks.Value{
				ks.NewValue(ks.NewTuple(ks.String, ks.String), []ks.Value{
					ks.NewValue(ks.String, "a"),
					ks.NewValue(ks.String, "some value"),
				}),
				ks.NewValue(ks.NewTuple(ks.String, ks.String), []ks.Value{
					ks.NewValue(ks.String, "b"),
					ks.NewValue(ks.Int, 10),
				}),
			}),
			output: encoded{
				val: `{"a":"some value","b":10}`,
				typ: `(sometype)`,
			},
		},
		{
			name: "nested object",
			input: ks.NewValue(user, map[string]any{
				"login": "i4k",
				"address": ks.NewValue(address, map[string]any{
					"street":  "unknown location",
					"number":  1337,
					"state":   "some",
					"country": "unknown",
				}),
				"books": ks.NewValue(books, []ks.Value{
					ks.NewValue(book, map[string]any{
						"id":     "some-id",
						"title":  "The Winter King",
						"author": "Bernard Cornwell",
					}),
				}),
			}),
			output: encoded{
				val: `{"address":{"country":"unknown","number":1337,"state":"some","street":"unknown location"},"books":[{"author":"Bernard Cornwell","id":"some-id","title":"The Winter King"}],"login":"i4k"}`,
				typ: `{"login":(string),"address":{"street":(string),"number":(int),"state":(string),"country":(string)},"books":[{"id":(string),"title":(string),"author":(string)}]}`,
			},
		},
	} {
		t.Run(tc.name+"/encoder", func(t *testing.T) {
			var got bytes.Buffer
			enc := ks.NewEncoder(&got)
			err := enc.EncodeValue(tc.input)
			if !errors.Is(err, tc.err) {
				t.Fatal(err)
			}
			if err != nil {
				return
			}
			assert(t, tc.output.val, got.String())
		})
		t.Run(tc.name+"/type", func(t *testing.T) {
			got, err := ks.Type(tc.input.Type()).MarshalText()
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
