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
		root func(*ast.Tree) ast.NodeID
		want ks.Value
	}
	z := token.UnknownSpan()
	for _, tc := range []testcase{
		{
			name: "type sum contains a list type",
			root: func(a *ast.Tree) ast.NodeID {
				return a.AddSchema(a.AddSum(
					a.AddList([]ast.NodeID{a.AddInt("10", z)}, z),
					a.AddName("string", z), z), nil, z)
			},
			want: ks.Sum(ks.With(ks.List(ks.LitInt(10))), ks.String()),
		},
		{
			name: "value sum contains a literal tuple",
			root: func(a *ast.Tree) ast.NodeID {
				return a.AddSum(
					a.AddList([]ast.NodeID{a.AddInt("10", z)}, z),
					a.AddSchema(a.AddName("string", z), nil, z), z)
			},
			want: ks.Sum(ks.List(ks.LitInt(10)), ks.String()),
		},
		{
			name: "nested type sum retains list context",
			root: func(a *ast.Tree) ast.NodeID {
				return a.AddSchema(a.AddSum(
					a.AddSum(a.AddList([]ast.NodeID{a.AddInt("10", z)}, z),
						a.AddName("string", z), z),
					a.AddName("bool", z), z), nil, z)
			},
			want: ks.Sum(ks.With(ks.List(ks.LitInt(10))), ks.String(), ks.Bool()),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tree := ast.New()
			root := tc.root(tree)
			arena := NewArena()
			got, err := Compile(arena, tree, root)
			if err != nil {
				t.Fatal(err)
			}
			if want := compile(t, arena, tc.want); got != want {
				t.Fatalf("compiled type = %d, want %d", got, want)
			}
		})
	}
}

func TestCompileListOfSums(t *testing.T) {
	t.Parallel()
	arena := NewArena()
	typ := compile(t, arena, ks.List(ks.Sum(ks.Int(), ks.String())))
	if got := arena.Type(typ).Kind(); got != List {
		t.Fatalf("kind = %v, want List", got)
	}
	type testcase struct {
		value ks.Value
		want  bool
	}
	for _, tc := range []testcase{
		{value: ks.List(), want: true},
		{value: ks.List(ks.LitInt(1), ks.LitString("a")), want: true},
		{value: ks.List(ks.LitBool(true)), want: false},
	} {
		tree, root, err := ks.Build(tc.value)
		if err != nil {
			t.Fatal(err)
		}
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
	aliasTree, aliasRoot, err := ks.Build(ks.Sum(ks.Int(), ks.String()))
	if err != nil {
		t.Fatal(err)
	}
	metadataTree, metadataRoot, err := ks.Build(ks.With(ks.Sum(ks.Int(), ks.String()), ks.Flag("owner")))
	if err != nil {
		t.Fatal(err)
	}
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
	typ := compile(t, arena, ks.Sum(
		ks.Where(ks.Int(), ks.Binary(ks.X(), ks.Gt, ks.IntExpr(0))),
		ks.Where(ks.String(), ks.Binary(ks.Funcall("len", ks.X()), ks.Gt, ks.IntExpr(0))),
	))
	type testcase struct {
		value ks.Value
		want  bool
	}
	for _, tc := range []testcase{
		{value: ks.LitInt(1), want: true},
		{value: ks.LitInt(0), want: false},
		{value: ks.LitString("a"), want: true},
		{value: ks.LitString(""), want: false},
	} {
		tree, root, err := ks.Build(tc.value)
		if err != nil {
			t.Fatal(err)
		}
		if got := arena.Valid(typ, tree, root); got != tc.want {
			t.Fatalf("Valid = %v, want %v", got, tc.want)
		}
	}
}
