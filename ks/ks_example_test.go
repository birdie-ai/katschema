package ks_test

import (
	"os"

	"github.com/birdie-ai/katschema/ks"
	. "github.com/birdie-ai/katschema/ks"
	"github.com/birdie-ai/katschema/parser/ast"
)

func ExampleBuild() {
	tree, root, err := Build(ks.Object(
		Field("id", Type("uuid")),
		Field("name", String()),
		Field("role", Where(String(),
			Binary(X(), In, ListExpr(LitString("admin"), LitString("user"), LitString("guest"))),
		)),
		Field("age", Optional(Where(Int(),
			Binary(X(), Ge, IntExpr(LitInt("18"))),
		))),
	))
	if err != nil {
		panic(err)
	}
	if err := ast.Print(os.Stdout, tree, root); err != nil {
		panic(err)
	}
	// Output: {"id":(uuid),"name":(string),"role":(string,x in ["admin","user","guest"]),"age":(int,x>=18,optional)}
}
