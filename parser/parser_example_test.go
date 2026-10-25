package parser_test

import (
	"fmt"
	"os"

	"github.com/birdie-ai/katschema/parser"
	"github.com/birdie-ai/katschema/parser/ast"
)

func ExampleParse() {
	src := `{
		// Service configuration.
		"service": "catalog",
		"version": 2,
		"enabled": true,
		"owner": ({
			"name": (string),
			"email": (string),
		}),
		"endpoints": ([{
			"path": (string),
			"methods": [(string)],
			"timeout": (real),
		}]),
		"defaults": {
			"retries": 3,
			"timeout": 2.5e+1,
			"fallback": null,
		},
	}`
	tree, root, err := parser.Parse(src)
	if err != nil {
		panic(err)
	}
	if err := ast.Print(os.Stdout, tree, root); err != nil {
		panic(err)
	}
	fmt.Printf("\nComments: %d\n", len(tree.Comments()))
	// Output:
	// {"service":"catalog","version":2,"enabled":true,"owner":({"name":(string),"email":(string)}),"endpoints":([{"path":(string),"methods":[(string)],"timeout":(real)}]),"defaults":{"retries":3,"timeout":2.5e+1,"fallback":null}}
	// Comments: 1
}
