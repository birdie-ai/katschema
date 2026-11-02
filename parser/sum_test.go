package parser

import (
	"testing"

	"github.com/birdie-ai/katschema/internal/compiled"
	"github.com/birdie-ai/katschema/ks"
	"github.com/google/go-cmp/cmp"
)

func TestParseSums(t *testing.T) {
	t.Parallel()
	type testcase struct {
		src  string
		want string
	}
	for _, tc := range []testcase{
		{src: "(string | int)", want: "(string|int)"},
		{src: "(string) | (int)", want: "(string)|(int)"},
		{src: "(string | int | bool)", want: "(string|int|bool)"},
		{src: "(string) | (int) | (bool)", want: "(string)|(int)|(bool)"},
		{src: "((string | int) | bool)", want: "((string|int)|bool)"},
		{src: "(string | (int | bool))", want: "(string|(int|bool))"},
		{src: "((string) | (int), optional,)", want: "((string)|(int),optional)"},
		{src: `(string | (int, label = "number"), label = "value")`, want: `(string|(int,label="number"),label="value")`},
		{src: `({"name":(string)} | [(int)])`, want: `({"name":(string)}|[(int)])`},
		{src: "(.types.name | pkg.id)", want: "(.types.name|pkg.id)"},
		{src: `{"value":(string | int, optional),"items":[(string) | (int)]}`, want: `{"value":(string|int,optional),"items":[(string)|(int)]}`},
		{src: "(string | int, choices=[(bool) | (int)])", want: "(string|int,choices=[(bool)|(int)])"},
	} {
		t.Run(tc.src, func(t *testing.T) {
			tree, root := parseOK(t, tc.src)
			if diff := cmp.Diff(tc.want, printString(t, tree, root)); diff != "" {
				t.Fatalf("printed sum differs (-want +got):\n%s", diff)
			}
			checkParsePrint(t, tree, root)
		})
	}
}

func TestParseSumErrors(t *testing.T) {
	t.Parallel()
	type testcase struct {
		src string
	}
	for _, tc := range []testcase{
		{src: "(int |)"},
		{src: "(| int)"},
		{src: "(int || string)"},
		{src: "(int | | string)"},
		{src: "(int) |"},
		{src: "| (int)"},
		{src: "(int) || (string)"},
		{src: "(int) | string"},
		{src: "int | string"},
		{src: "1 | 2"},
		{src: "(int | 1)"},
		{src: "(int | null)"},
		{src: "(int) | null"},
		{src: "(int | string, optional | bool)"},
		{src: "(int | string,,)"},
		{src: "(int | string, label=)"},
		{src: "((int))"},
		{src: "((int) |)"},
		{src: "(int | string, x > 0)"},
		{src: "(string, len(x) > 0) | (int, x > 0)"},
		{src: "(int | #{1})"},
	} {
		t.Run(tc.src, func(t *testing.T) {
			parseFailure(t, tc.src)
		})
	}
}

func TestParseSumCompile(t *testing.T) {
	t.Parallel()
	type testcase struct {
		src     string
		dsl     ks.Value
		valid   []string
		invalid []string
	}
	for _, tc := range []testcase{
		{
			src:     "(string | int)",
			dsl:     ks.Sum(ks.String(), ks.Int()),
			valid:   []string{`"text"`, "0", "-10"},
			invalid: []string{"null", "true", "1.5", "[]"},
		},
		{
			src:     "(string) | (int)",
			dsl:     ks.Sum(ks.String(), ks.Int()),
			valid:   []string{`"text"`, "0"},
			invalid: []string{"false", "{}"},
		},
		{
			src:     `(string | int, label="value")`,
			dsl:     ks.With(ks.Sum(ks.String(), ks.Int()), ks.Attr("label", ks.StrExpr("value"))),
			valid:   []string{`"text"`, "0"},
			invalid: []string{"null"},
		},
		{
			src:     `(string,label="text") | (int,label="number")`,
			dsl:     ks.Sum(ks.With(ks.String(), ks.Attr("label", ks.StrExpr("text"))), ks.With(ks.Int(), ks.Attr("label", ks.StrExpr("number")))),
			valid:   []string{`"text"`, "0"},
			invalid: []string{"null"},
		},
		{
			src:     `{"value":(string | int, optional)}`,
			dsl:     ks.Object(ks.Field("value", ks.Optional(ks.Sum(ks.String(), ks.Int())))),
			valid:   []string{"{}", `{"value":1}`, `{"value":"text"}`},
			invalid: []string{`{"value":true}`, `{"value":null}`},
		},
		{
			src:     `{"value":((string) | (int), optional)}`,
			dsl:     ks.Object(ks.Field("value", ks.Optional(ks.Sum(ks.String(), ks.Int())))),
			valid:   []string{"{}", `{"value":1}`},
			invalid: []string{`{"value":false}`},
		},
		{
			src:     "[(string) | (int)]",
			dsl:     ks.List(ks.Sum(ks.String(), ks.Int())),
			valid:   []string{"[]", `[1,"text",2]`},
			invalid: []string{"1", "[null]", "[true]"},
		},
		{
			src:     "([1] | string)",
			dsl:     ks.Sum(ks.With(ks.List(ks.LitInt(1))), ks.String()),
			valid:   []string{"[]", "[1,1]", `"text"`},
			invalid: []string{"[2]", "[1,2]", "1"},
		},
		{
			src:     `({"id":(int)} | [(string)])`,
			dsl:     ks.Sum(ks.Object(ks.Field("id", ks.Int())), ks.List(ks.String())),
			valid:   []string{`{"id":1}`, "[]", `["a","b"]`},
			invalid: []string{"{}", `{"id":"1"}`, "[1]"},
		},
		{
			src:     "(string | (int | bool))",
			dsl:     ks.Sum(ks.String(), ks.Int(), ks.Bool()),
			valid:   []string{`"text"`, "1", "false"},
			invalid: []string{"null", "[]"},
		},
	} {
		t.Run(tc.src, func(t *testing.T) {
			tree, root := parseOK(t, tc.src)
			arena := compiled.NewArena()
			got, err := compiled.Compile(arena, tree, root)
			if err != nil {
				t.Fatal(err)
			}
			dslTree, dslRoot, err := ks.Build(tc.dsl)
			if err != nil {
				t.Fatal(err)
			}
			want, err := compiled.Compile(arena, dslTree, dslRoot)
			if err != nil || got != want {
				t.Fatalf("parsed type = %v, DSL type = %v, error = %v", got, want, err)
			}
			for _, src := range tc.valid {
				values, value := parseOK(t, src)
				if !arena.Valid(got, values, value) {
					t.Errorf("sum rejected %s", src)
				}
			}
			for _, src := range tc.invalid {
				values, value := parseOK(t, src)
				if arena.Valid(got, values, value) {
					t.Errorf("sum accepted %s", src)
				}
			}
		})
	}
}
