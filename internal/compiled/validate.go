package compiled

import "github.com/birdie-ai/katschema/parser/ast"

func (a *Arena) Valid(typ TypeID, values *ast.Tree, value ast.NodeID) bool {
	return a.Type(typ).Valid(values, value)
}

// Valid compiles the value in a temporary arena before validation.
func (typ Type) Valid(tree *ast.Tree, root ast.NodeID) bool {
	values := NewArena()
	id, err := Compile(values, tree, root)
	return err == nil && typ.ValidType(values.Type(id))
}
