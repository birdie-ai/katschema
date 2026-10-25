package parser

import (
	"bytes"
	"errors"
	"testing"

	"github.com/birdie-ai/katschema/internal/compiled"
	"github.com/birdie-ai/katschema/ks"
	"github.com/birdie-ai/katschema/parser/ast"
	"github.com/google/go-cmp/cmp"
)

func TestParseBasicSchemas(t *testing.T) {
	t.Parallel()

	type testcase struct {
		src  string
		kind ast.Kind
		name string
	}

	for _, tc := range []testcase{
		{src: "(string)", kind: ast.Name, name: "string"},
		{src: "(int)", kind: ast.Name, name: "int"},
		{src: "(unknown)", kind: ast.Name, name: "unknown"},
		{src: "(λ2.牦牛)", kind: ast.Name, name: "λ2.牦牛"},
		{src: "(pkg /* one */ . true . false . null . in)", kind: ast.Name, name: "pkg.true.false.null.in"},
		{src: "(.account.id)", kind: ast.Path},
		{src: "(.true.false.null.in)", kind: ast.Path},
		{src: "([])", kind: ast.List},
		{src: "([1,(int)])", kind: ast.List},
		{src: `({"name":(string)})`, kind: ast.Object},
	} {
		t.Run(tc.src, func(t *testing.T) {
			tree, root := parseOK(t, tc.src)
			if tree.Node(root).Kind() != ast.Schema {
				t.Fatalf("root kind = %v, want Schema", tree.Node(root).Kind())
			}
			schema := tree.Schema(root)
			if len(schema.Clauses) != 0 || tree.Node(schema.Type).Kind() != tc.kind {
				t.Fatalf("schema = %#v, type kind = %v, want %v without clauses", schema, tree.Node(schema.Type).Kind(), tc.kind)
			}
			if tc.kind == ast.Name && tree.Name(schema.Type) != tc.name {
				t.Fatalf("type name = %q, want %q", tree.Name(schema.Type), tc.name)
			}
			checkParsePrint(t, tree, root)
		})
	}
}

func TestParseKatschemaExtensions(t *testing.T) {
	t.Parallel()

	type testcase struct {
		src string
	}

	for _, tc := range []testcase{
		{src: `[1,]`},
		{src: `{"a":1,}`},
		{src: `{"name":(string),"age":(int)}`},
		{src: `[null,{"name":(string)},[(int)],]`},
		{src: "/* start */ {\"x\" /* key */ : /* value */ [1, /* end */],} // tail"},
		{src: "\v\f\u00a0[true]\u2003"},
	} {
		t.Run(tc.src, func(t *testing.T) {
			tree, root := parseOK(t, tc.src)
			checkParsePrint(t, tree, root)
		})
	}
}

func TestParseErrors(t *testing.T) {
	t.Parallel()

	type testcase struct {
		src string
	}

	for _, tc := range []testcase{
		{src: ""}, {src: " \t\r\n"}, {src: "// only a comment"}, {src: "/* only a comment */"},
		{src: "1true"}, {src: "0false"}, {src: "true null"}, {src: "{}[]"}, {src: `"a""b"`},
		{src: "[0,1true]"}, {src: "[1 true]"}, {src: "[,1]"}, {src: "[1,,]"}, {src: "[,]"},
		{src: "["}, {src: "[1"}, {src: "[1}"}, {src: "{]"}, {src: "}"},
		{src: `{"a":1 "b":2}`}, {src: `{"a" 1}`}, {src: `{"a":}`}, {src: `{a:1}`},
		{src: `{,"a":1}`}, {src: `{"a":1,,}`}, {src: `{,}`}, {src: `{"a":1,`},
		{src: "+1"}, {src: "-"}, {src: "--1"}, {src: "- 1"}, {src: "-/**/1"},
		{src: "01"}, {src: "-01"}, {src: "1."}, {src: ".1"}, {src: "1e"}, {src: "1e+"},
		{src: "NaN"}, {src: "Infinity"}, {src: "string"}, {src: "in"}, {src: "True"},
		{src: `"\q"`}, {src: `"\u123"`}, {src: "\"a\n\""}, {src: "\"\x00\""}, {src: "\""},
		{src: "\xff"}, {src: "/* unterminated"}, {src: "0 /* unterminated"},
		{src: "()"}, {src: "(int"}, {src: "(int,)"}, {src: "(in)"}, {src: "(int int)"},
		{src: "(true)"}, {src: "(false)"}, {src: "(null)"}, {src: "(-0)"}, {src: "(1.20e+3)"},
		{src: `("literal")`}, {src: "((int))"},
		{src: "(.)"}, {src: "(name.)"}, {src: "(.a.)"}, {src: "(a..b)"}, {src: "(a.1)"},
		{src: "(int, optional)"}, {src: "(int, pk)"}, {src: `(string, label="Name")`},
		{src: "(int, x > 0)"}, {src: "(string, len(x))"}, {src: "(int, 0 <= x <= 9)"},
		{src: "(int | string)"}, {src: "(int) | (string)"}, {src: "[(int) | (string)]"},
		{src: "#{1}"}, {src: "(#{(int)})"}, {src: "[#{1}]"},
	} {
		t.Run(tc.src, func(t *testing.T) {
			parseFailure(t, tc.src)
		})
	}
}

func TestParseCompile(t *testing.T) {
	t.Parallel()

	type testcase struct {
		src     string
		dsl     ks.Value
		valid   []string
		invalid []string
	}

	for _, tc := range []testcase{
		{
			src:     "(int)",
			dsl:     ks.Int(),
			valid:   []string{"0", "-1"},
			invalid: []string{`"1"`, "null", "true"},
		},
		{
			src:     "(string)",
			dsl:     ks.String(),
			valid:   []string{`""`, `"hello"`},
			invalid: []string{"1", "[]"},
		},
		{
			src:     "[(int)]",
			dsl:     ks.List(ks.Int()),
			valid:   []string{"[]", "[1,2]"},
			invalid: []string{`["1"]`, "1"},
		},
		{
			src: `{"name":(string),"age":(int)}`,
			dsl: ks.Object(
				ks.Field("name", ks.String()),
				ks.Field("age", ks.Int()),
			),
			valid:   []string{`{"name":"ken","age":83}`},
			invalid: []string{`{"name":"i4k"}`, `{"name":"i4k","age":"38"}`},
		},
		{
			src: `{"team":[{"name":(string)}]}`,
			dsl: ks.Object(
				ks.Field("team", ks.List(ks.Object(ks.Field("name", ks.String())))),
			),
			valid:   []string{`{"team":[]}`, `{"team":[{"name":"plan9"}]}`},
			invalid: []string{`{"team":[{"name":1}]}`},
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
					t.Errorf("schema rejected %s", src)
				}
			}
			for _, src := range tc.invalid {
				values, value := parseOK(t, src)
				if arena.Valid(got, values, value) {
					t.Errorf("schema accepted %s", src)
				}
			}
		})
	}
	tree, root := parseOK(t, "(unknown)")
	if _, err := compiled.Compile(compiled.NewArena(), tree, root); !errors.Is(err, compiled.ErrUnknownType) {
		t.Fatalf("unknown type error = %v", err)
	}
}

func parseOK(t testing.TB, src string) (*ast.Tree, ast.NodeID) {
	t.Helper()

	tree, root, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse failed for %d bytes: %v", len(src), err)
	}
	if tree == nil || root == 0 {
		t.Fatal("successful Parse returned no tree or root")
	}
	return tree, root
}

func parseFailure(t testing.TB, src string) *Error {
	t.Helper()

	tree, root, err := Parse(src)
	if err == nil || tree != nil || root != 0 {
		t.Fatalf("Parse(%q) = (%p, %d, %v), want (nil, 0, error)", src, tree, root, err)
	}
	var syntax *Error
	if !errors.As(err, &syntax) || syntax.Message == "" || !syntax.Span.Valid() || syntax.Span.End.Offset() > len(src) {
		t.Fatalf("invalid syntax error: %#v", err)
	}
	return syntax
}

func checkParsePrint(t testing.TB, tree *ast.Tree, root ast.NodeID) {
	t.Helper()

	var want bytes.Buffer
	if err := ast.Print(&want, tree, root); err != nil {
		t.Fatal(err)
	}
	got, gotRoot := parseOK(t, want.String())
	var printed bytes.Buffer
	if err := ast.Print(&printed, got, gotRoot); err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want.String(), printed.String()); diff != "" {
		t.Fatalf("printed text changed after parsing (-want +got):\n%s", diff)
	}
}
