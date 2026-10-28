package katschema_test

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"testing"

	"github.com/birdie-ai/katschema"
	"github.com/birdie-ai/katschema/ks"
	"github.com/birdie-ai/katschema/parser"
	"github.com/birdie-ai/katschema/parser/ast"
	"github.com/birdie-ai/katschema/parser/token"
)

func parseAST(t *testing.T, source string) (*ast.Tree, ast.NodeID) {
	t.Helper()
	tree, root, err := parser.Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	return tree, root
}

func TestCompileAST(t *testing.T) {
	t.Parallel()

	large := new(big.Int).Lsh(big.NewInt(1), 100)
	type testcase struct {
		name   string
		source string
		value  ks.Value
	}
	for _, tc := range []testcase{
		{"literal", `{"items": [null, true, "é", 1267650600228229401496703205376, 9007199254740993.125], "empty": {}}`,
			ks.Object(ks.Field("items", ks.List(ks.LitNull(), ks.LitBool(true), ks.LitString("é"), ks.LitBigInt(large), ks.LitDecimal("9007199254740993.125"))), ks.Field("empty", ks.Object()))},
		{"schema", `{"id": (string), "tags": [(string)]}`, ks.Object(ks.Field("id", ks.String()), ks.Field("tags", ks.List(ks.String())))},
		{"empty list", `[]`, ks.List()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tree, root := parseAST(t, tc.source)
			want := compile(t, katschema.NewCompiler(), tc.value)
			cc := katschema.NewCompiler()
			for name, compileAST := range map[string]func(context.Context, *ast.Tree, ast.NodeID) (katschema.Type, error){
				"standalone":      katschema.CompileAST,
				"shared compiler": cc.CompileAST,
			} {
				got, err := compileAST(t.Context(), tree, root)
				if err != nil {
					t.Fatalf("%s: %v", name, err)
				}
				if !got.Equivalent(want) || got.Fingerprint() != want.Fingerprint() {
					t.Errorf("%s: CompileAST differs from Compile", name)
				}
			}
		})
	}
}

func TestCompileASTSubtrees(t *testing.T) {
	t.Parallel()

	tree, root := parseAST(t, `[{"id": "one", "score": 1}, {"id": "two", "score": 2}]`)
	cc := katschema.NewCompiler()
	schema := compile(t, cc, ks.Object(ks.Field("id", ks.String()), ks.Field("score", ks.Int())))
	var values []katschema.Type
	for _, node := range tree.List(root) {
		if err := schema.ValidateAST(tree, node); err != nil {
			t.Fatal(err)
		}
		value, err := cc.CompileAST(t.Context(), tree, node)
		if err != nil {
			t.Fatal(err)
		}
		values = append(values, value)
	}

	// NOTE(i4k): compiled values must not retain references to the reusable AST's storage.
	tree.Reset()
	span := token.UnknownSpan()
	replacementRoot := tree.AddObject([]ast.Field{
		tree.NewField("id", tree.AddString("replacement", span), span, span),
		tree.NewField("score", tree.AddInt("3", span), span, span),
	}, span)
	if _, err := cc.CompileAST(t.Context(), tree, replacementRoot); err != nil {
		t.Fatal(err)
	}
	for i, want := range []string{"one", "two"} {
		if err := schema.ValidateType(values[i]); err != nil {
			t.Fatal(err)
		}
		field, ok := values[i].Field("id")
		if !ok {
			t.Fatal("compiled subtree lost its id field")
		}
		if got, ok := field.Type.StringValue(); !ok || got != want {
			t.Fatalf("subtree %d id = %q, %v; want %q", i, got, ok, want)
		}
	}
}

func TestCompileASTResolver(t *testing.T) {
	t.Parallel()

	type contextKey struct{}
	calls := 0
	cc := katschema.NewCompilerWithResolver(katschema.ResolverFunc(func(ctx context.Context, name string) (katschema.TypeResolution, error) {
		calls++
		if name != "record" {
			return katschema.TypeResolution{}, katschema.ErrUnknownType
		}
		tenant, ok := ctx.Value(contextKey{}).(string)
		if !ok {
			t.Fatal("CompileAST did not pass its context to the resolver")
		}
		fieldType := ks.String()
		if calls > 2 {
			// Reusing a scope key must reuse its compiled definition even if
			// a resolver returns a different definition on a later call.
			fieldType = ks.Int()
		}
		return katschema.TypeResolution{
			Value: ks.Object(ks.Field(tenant, fieldType)), Name: "record", CacheKey: tenant,
		}, nil
	}))
	tree, root := parseAST(t, `(record)`)
	first, err := cc.CompileAST(context.WithValue(t.Context(), contextKey{}, "first"), tree, root)
	if err != nil {
		t.Fatal(err)
	}
	second, err := cc.CompileAST(context.WithValue(t.Context(), contextKey{}, "second"), tree, root)
	if err != nil {
		t.Fatal(err)
	}
	firstAgain, err := cc.Compile(context.WithValue(t.Context(), contextKey{}, "first"), ks.Type("record"))
	if err != nil {
		t.Fatal(err)
	}
	if calls != 3 {
		t.Fatalf("resolver calls = %d, want 3 to obtain each compilation's scope key", calls)
	}
	if first.Name() != "record" || second.Name() != "record" {
		t.Fatal("CompileAST lost the resolved type name")
	}
	if first.Equivalent(second) || !first.Equivalent(firstAgain) || first.Fingerprint() != firstAgain.Fingerprint() {
		t.Fatal("CompileAST and Compile did not share the compiler's scoped resolution cache")
	}
	missingTree, missingRoot := parseAST(t, `(missing)`)
	if _, err := cc.CompileAST(t.Context(), missingTree, missingRoot); !errors.Is(err, katschema.ErrUnknownType) {
		t.Fatalf("unknown type error = %v, want ErrUnknownType", err)
	}
}

func TestASTInvalidInput(t *testing.T) {
	t.Parallel()

	schema := compile(t, katschema.NewCompiler(), ks.Any())
	type testcase struct {
		name string
		tree *ast.Tree
		root ast.NodeID
	}
	for _, tc := range []testcase{
		{"zero root", ast.New(), 0},
		{"missing root", ast.New(), 100},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := katschema.CompileAST(t.Context(), tc.tree, tc.root); err == nil {
				t.Error("CompileAST accepted an invalid AST root")
			}
			if _, err := katschema.NewCompiler().CompileAST(t.Context(), tc.tree, tc.root); err == nil {
				t.Error("Compiler.CompileAST accepted an invalid AST root")
			}
			if err := schema.ValidateAST(tc.tree, tc.root); !errors.Is(err, katschema.ErrValidation) {
				t.Errorf("ValidateAST = %v, want ErrValidation", err)
			}
		})
	}
	tree, root := parseAST(t, `null`)
	if err := (katschema.Type{}).ValidateAST(tree, root); !errors.Is(err, katschema.ErrValidation) {
		t.Fatalf("zero schema ValidateAST = %v, want ErrValidation", err)
	}
}

func TestValidateASTMatchesCompiledLiterals(t *testing.T) {
	t.Parallel()

	large := new(big.Int).Lsh(big.NewInt(1), 100)
	lengthOne := ks.Binary(
		ks.Binary(ks.Funcall("len", ks.X()), ks.Ge, ks.IntExpr(1)),
		ks.And,
		ks.Binary(ks.Funcall("len", ks.X()), ks.Le, ks.IntExpr(1)),
	)
	object := ks.Object(ks.Field("id", ks.String()), ks.Field("tags", ks.Optional(ks.List(ks.String()))))

	type testcase struct {
		name   string
		schema ks.Value
		source string
		valid  bool
	}

	for _, tc := range []testcase{
		{"nested any", ks.Any(), `{"items": [null, true, {}, [], 1.25]}`, true},
		{"large integer", ks.LitBigInt(large), `1267650600228229401496703205376`, true},
		{"neighboring integer", ks.LitBigInt(large), `1267650600228229401496703205377`, false},
		{"integer as real", ks.Real(), `1267650600228229401496703205376`, true},
		{"exact decimal", ks.LitDecimal("9007199254740993.125"), `9007199254740993.1250`, true},
		{"neighboring decimal", ks.LitDecimal("9007199254740993.125"), `9007199254740993.126`, false},
		{"large exponent", ks.Real(), `1e1000000`, true},
		{"decimal as integer", ks.Int(), `1.0`, false},
		{"bounded integer", ks.Int8(), `128`, false},
		{"unicode length", ks.Where(ks.String(), lengthOne), `"é"`, true},
		{"unicode too long", ks.Where(ks.String(), lengthOne), `"éø"`, false},
		{"list length", ks.Where(ks.List(ks.Int()), lengthOne), `[1]`, true},
		{"empty list length", ks.Where(ks.List(ks.Int()), lengthOne), `[]`, false},
		{"optional omitted", object, `{"id": "one"}`, true},
		{"unordered fields", object, `{"tags": ["a", "b"], "id": "one"}`, true},
		{"wrong element", object, `{"id": "one", "tags": ["a", 2]}`, false},
		{"missing required", object, `{"tags": []}`, false},
		{"unknown field", object, `{"id": "one", "internal": true}`, false},
		{"object length", ks.Where(ks.Object(ks.Field("id", ks.Optional(ks.Int()))), lengthOne), `{"id": 1}`, true},
		{"empty object length", ks.Where(ks.Object(ks.Field("id", ks.Optional(ks.Int()))), lengthOne), `{}`, false},
		{"tuple", ks.List(ks.LitInt(1), ks.LitString("a")), `[1, "a"]`, true},
		{"tuple order", ks.List(ks.LitInt(1), ks.LitString("a")), `["a", 1]`, false},
		{"sum", ks.Sum(ks.Int(), ks.String()), `"one"`, true},
		{"sum mismatch", ks.Sum(ks.Int(), ks.String()), `false`, false},
		{"never", ks.Never(), `null`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			schema := compile(t, katschema.NewCompiler(), tc.schema)
			tree, root := parseAST(t, tc.source)
			value, err := katschema.CompileAST(t.Context(), tree, root)
			if err != nil {
				t.Fatal(err)
			}
			for name, err := range map[string]error{
				"ValidateAST":  schema.ValidateAST(tree, root),
				"ValidateType": schema.ValidateType(value),
			} {
				if (err == nil) != tc.valid {
					t.Errorf("%s = %v, want valid=%v", name, err, tc.valid)
				}
				if err != nil && !errors.Is(err, katschema.ErrValidation) {
					t.Errorf("%s error does not wrap ErrValidation: %v", name, err)
				}
			}
		})
	}
}

func TestValidateASTSchemaExpressions(t *testing.T) {
	t.Parallel()

	singleton := ks.Where(ks.Int(), ks.Binary(ks.X(), ks.Eq, ks.IntExpr(1)))

	type testcase struct {
		name   string
		schema ks.Value
		value  ks.Value
		valid  bool
	}

	for _, tc := range []testcase{
		{"singleton", ks.Any(), singleton, true},
		{"nested singleton", ks.Any(), ks.Object(ks.Field("id", singleton)), true},
		{"typed nested singleton", ks.Object(ks.Field("id", ks.Int())), ks.Object(ks.Field("id", singleton)), true},
		{"optional singleton", ks.Any(), ks.Object(ks.Field("id", ks.Optional(singleton))), false},
		{"typed optional singleton", ks.Object(ks.Field("id", ks.Optional(ks.Int()))), ks.Object(ks.Field("id", ks.Optional(singleton))), false},
		{"nested schema", ks.Any(), ks.Object(ks.Field("id", ks.Int())), false},
		{"nested never", ks.Any(), ks.Object(ks.Field("id", ks.Never())), false},
		{"list schema", ks.Any(), ks.List(ks.Int()), false},
		{"singleton list schema", ks.Any(), ks.List(singleton), false},
		{"nested singleton list schema", ks.Any(), ks.List(ks.Object(ks.Field("id", singleton))), false},
		{"sum singleton list value", ks.Any(), ks.List(ks.Sum(singleton, singleton)), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			schema := compile(t, katschema.NewCompiler(), tc.schema)
			tree, root, err := ks.Build(tc.value)
			if err != nil {
				t.Fatal(err)
			}
			value, err := katschema.CompileAST(t.Context(), tree, root)
			if err != nil {
				t.Fatal(err)
			}
			for name, err := range map[string]error{"ValidateAST": schema.ValidateAST(tree, root), "ValidateType": schema.ValidateType(value)} {
				if (err == nil) != tc.valid {
					t.Errorf("%s = %v, want valid=%v", name, err, tc.valid)
				}
			}
		})
	}
}

func TestValidateASTInvalidNumbers(t *testing.T) {
	t.Parallel()

	schema := compile(t, katschema.NewCompiler(), ks.Any())
	type testcase struct {
		kind  ast.Kind
		raw   string
		valid bool
	}
	for _, tc := range []testcase{
		{ast.Int, "invalid", false},
		{ast.Int, "1.5", false},
		{ast.Decimal, "invalid", false},
		{ast.Decimal, "1e", false},
		{ast.Int, "+1", true},
		{ast.Int, "01", true},
	} {
		t.Run(fmt.Sprintf("%s/%s", tc.kind, tc.raw), func(t *testing.T) {
			t.Parallel()
			tree := ast.New()
			span := token.UnknownSpan()
			var number ast.NodeID
			if tc.kind == ast.Int {
				number = tree.AddInt(tc.raw, span)
			} else {
				number = tree.AddDecimal(tc.raw, span)
			}
			root := tree.AddObject([]ast.Field{tree.NewField("number", number, span, span)}, span)
			value, compileErr := katschema.CompileAST(t.Context(), tree, root)
			if (compileErr == nil) != tc.valid {
				t.Fatalf("CompileAST = %v, want valid=%v", compileErr, tc.valid)
			}
			if err := schema.ValidateAST(tree, root); (err == nil) != tc.valid {
				t.Errorf("ValidateAST = %v, want valid=%v", err, tc.valid)
			}
			if tc.valid {
				if err := schema.ValidateType(value); err != nil {
					t.Errorf("ValidateType = %v", err)
				}
			}
		})
	}
}

func TestValidateASTWideObject(t *testing.T) {
	t.Parallel()

	var schemaFields, valueFields []ks.Member
	for i := range 80 {
		name := fmt.Sprintf("field%02d", i)
		fieldType := ks.Int()
		if i == 78 {
			fieldType = ks.Optional(fieldType)
		}
		schemaFields = append(schemaFields, ks.Field(name, fieldType))
		valueFields = append(valueFields, ks.Field(name, ks.LitInt(int64(i))))
	}
	schema := compile(t, katschema.NewCompiler(), ks.Object(schemaFields...))
	anyType := compile(t, katschema.NewCompiler(), ks.Any())
	type testcase struct {
		name       string
		fields     []ks.Member
		valid      bool
		validAsAny bool
	}
	for _, tc := range []testcase{
		{"complete", valueFields, true, true},
		{"missing required", valueFields[:79], false, true},
		{"optional omitted", append(append([]ks.Member{}, valueFields[:78]...), valueFields[79]), true, true},
		{"duplicate", append(append([]ks.Member{}, valueFields[:79]...), valueFields[0]), false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tree, root, err := ks.Build(ks.Object(tc.fields...))
			if err != nil {
				t.Fatal(err)
			}
			if err := schema.ValidateAST(tree, root); (err == nil) != tc.valid {
				t.Errorf("ValidateAST = %v, want valid=%v", err, tc.valid)
			}
			if err := anyType.ValidateAST(tree, root); (err == nil) != tc.validAsAny {
				t.Errorf("Any.ValidateAST = %v, want valid=%v", err, tc.validAsAny)
			}
		})
	}
}

func TestValidateASTRejectsDuplicateFields(t *testing.T) {
	t.Parallel()

	for _, source := range []string{`{"id": 1, "id": 2}`, `{"data": {"id": 1, "id": 2}}`, `[{"id": 1, "id": 2}]`} {
		t.Run(source, func(t *testing.T) {
			t.Parallel()
			tree, root := parseAST(t, source)
			for _, value := range []ks.Value{ks.Any(), ks.Object(ks.Field("id", ks.Optional(ks.Int())), ks.Field("data", ks.Optional(ks.Any())))} {
				schema := compile(t, katschema.NewCompiler(), value)
				if err := schema.ValidateAST(tree, root); !errors.Is(err, katschema.ErrValidation) {
					t.Errorf("ValidateAST duplicate fields = %v, want ErrValidation", err)
				}
			}
			if _, err := katschema.CompileAST(t.Context(), tree, root); err == nil {
				t.Error("CompileAST accepted duplicate fields")
			}
		})
	}
}

func TestValidateASTOverlay(t *testing.T) {
	t.Parallel()

	bottom := compile(t, katschema.NewCompiler(), ks.With(ks.Object(ks.Field("id", ks.String()), ks.Field("priority", ks.String())), ks.Flag("entity")))
	top := compile(t, katschema.NewCompiler(), ks.Object(ks.Field("priority", ks.Int())))
	schema, err := katschema.Overlay(bottom, top)
	if err != nil {
		t.Fatal(err)
	}
	type testcase struct {
		source string
		valid  bool
	}
	for _, tc := range []testcase{
		{`{"priority": 1, "id": "one"}`, true},
		{`{"priority": "wrong", "id": "one"}`, false},
		{`{"id": "one"}`, false},
	} {
		tree, root := parseAST(t, tc.source)
		if err := schema.ValidateAST(tree, root); (err == nil) != tc.valid {
			t.Errorf("ValidateAST(%s) = %v, want valid=%v", tc.source, err, tc.valid)
		}
	}
}
