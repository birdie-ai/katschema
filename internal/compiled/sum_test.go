package compiled

import (
	"errors"
	"testing"

	"github.com/birdie-ai/katschema/ks"
	"github.com/birdie-ai/katschema/parser/ast"
	"github.com/birdie-ai/katschema/parser/token"
	"github.com/google/go-cmp/cmp"
)

func TestCompileSumStructuralContext(t *testing.T) {
	t.Parallel()
	type testcase struct {
		name string
		src  string
		want ks.Value
	}
	for _, tc := range []testcase{
		{
			name: "type sum contains a list type",
			src:  "([10] | string)",
			want: ks.Sum(ks.With(ks.List(ks.LitInt(10))), ks.String()),
		},
		{
			name: "nested type sum retains list context",
			src:  "([10] | string | bool)",
			want: ks.Sum(ks.With(ks.List(ks.LitInt(10))), ks.String(), ks.Bool()),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			arena := NewArena()
			got := compileText(t, arena, tc.src)
			if want := compile(t, arena, tc.want); got != want {
				t.Fatalf("compiled type = %d, want %d", got, want)
			}
		})
	}
}

func TestCompileSumLiteralContext(t *testing.T) {
	t.Parallel()
	z := token.UnknownSpan()
	tree := ast.New()
	root := tree.AddSum(
		tree.AddList([]ast.NodeID{tree.AddInt("10", z)}, z),
		tree.AddSchema(tree.AddName("string", z), nil, z), z)
	arena := NewArena()
	got, err := Compile(arena, tree, root)
	if err != nil {
		t.Fatal(err)
	}
	if want := compile(t, arena, ks.Sum(ks.List(ks.LitInt(10)), ks.String())); got != want {
		t.Fatalf("compiled type = %d, want %d", got, want)
	}
}

func TestCompileListOfSums(t *testing.T) {
	t.Parallel()
	arena := NewArena()
	typ := compileText(t, arena, "[(int) | (string)]")
	if got := arena.Type(typ).Kind(); got != List {
		t.Fatalf("kind = %v, want List", got)
	}
	type testcase struct {
		value string
		want  bool
	}
	for _, tc := range []testcase{
		{value: "[]", want: true},
		{value: `[1, "a"]`, want: true},
		{value: "[true]", want: false},
	} {
		tree, root := parseText(t, tc.value)
		if got := arena.Valid(typ, tree, root); got != tc.want {
			t.Fatalf("Valid = %v, want %v", got, tc.want)
		}
	}
}

func TestCompileSumConstraints(t *testing.T) {
	t.Parallel()
	type testcase struct {
		name    string
		typ     func(*ast.Tree) ast.NodeID
		resolve TypeResolver
	}
	z := token.UnknownSpan()
	aliasTree, aliasRoot := parseText(t, "(int) | (string)")
	metadataTree, metadataRoot := parseText(t, "((int) | (string), owner)")
	for _, tc := range []testcase{
		{
			name: "direct sum",
			typ: func(a *ast.Tree) ast.NodeID {
				return a.AddSum(a.AddName("int", z), a.AddName("string", z), z)
			},
		},
		{
			name: "sum canonicalized to a single type",
			typ: func(a *ast.Tree) ast.NodeID {
				return a.AddSum(a.AddName("int", z), a.AddName("int", z), z)
			},
		},
		{
			name: "named sum",
			typ:  func(a *ast.Tree) ast.NodeID { return a.AddName("Choice", z) },
			resolve: func(string) (ResolvedType, error) {
				return ResolvedType{Tree: aliasTree, Root: aliasRoot}, nil
			},
		},
		{
			name: "named sum with metadata",
			typ:  func(a *ast.Tree) ast.NodeID { return a.AddName("Choice", z) },
			resolve: func(string) (ResolvedType, error) {
				return ResolvedType{Tree: metadataTree, Root: metadataRoot, Name: "Choice"}, nil
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tree := ast.New()
			span := token.NewSpan(20, 26)
			constraint := tree.AddConstraint(tree.AddBinary(
				tree.AddIdent("x", z), token.Eq, tree.AddInt("1", z), span), span)
			root := tree.AddSchema(tc.typ(tree), []ast.NodeID{constraint}, z)
			_, err := CompileWithResolver(NewArena(), tree, root, tc.resolve, nil)
			var compileErr *CompileError
			if !errors.As(err, &compileErr) {
				t.Fatalf("error = %v, want CompileError", err)
			}
			if got := compileErr.Err.Error(); got != "constraints must be applied to individual sum alternatives" {
				t.Fatalf("error = %q", got)
			}
			if diff := cmp.Diff(span, compileErr.Span); diff != "" {
				t.Fatalf("constraint span differs (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCompileSumBranchConstraints(t *testing.T) {
	t.Parallel()
	arena := NewArena()
	typ := compileText(t, arena, "(int, x > 0) | (string, len(x) > 0)")
	type testcase struct {
		value string
		want  bool
	}
	for _, tc := range []testcase{
		{value: "1", want: true},
		{value: "0", want: false},
		{value: `"a"`, want: true},
		{value: `""`, want: false},
	} {
		tree, root := parseText(t, tc.value)
		if got := arena.Valid(typ, tree, root); got != tc.want {
			t.Fatalf("Valid = %v, want %v", got, tc.want)
		}
	}
}
