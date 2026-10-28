package katschema_test

import (
	"context"
	"fmt"

	"github.com/birdie-ai/katschema"
	"github.com/birdie-ai/katschema/ks"
	"github.com/birdie-ai/katschema/parser"
)

func ExampleType_ValidateAST() {
	schema, err := katschema.Compile(context.Background(), ks.Object(ks.Field("id", ks.String())))
	if err != nil {
		panic(err)
	}
	tree, root, err := parser.Parse(`{"id":"one"}`)
	if err != nil {
		panic(err)
	}
	// Validation using the AST (no compilation required)
	fmt.Println(schema.ValidateAST(tree, root))
	// Output: <nil>
}

func ExampleCompiler_CompileAST() {
	ctx := context.Background()
	schema, err := katschema.Compile(ctx, ks.Object(ks.Field("id", ks.String())))
	if err != nil {
		panic(err)
	}
	tree, root, err := parser.Parse(`[{"id":"one"},{"id":"two"}]`)
	if err != nil {
		panic(err)
	}
	// One compiler owns this batch's values; the schema has its own arena.
	values := katschema.NewCompiler()
	for _, node := range tree.List(root) {
		value, err := values.CompileAST(ctx, tree, node)
		if err != nil {
			panic(err)
		}
		fmt.Println(schema.ValidateType(value))
	}
	// Output:
	// <nil>
	// <nil>
}
