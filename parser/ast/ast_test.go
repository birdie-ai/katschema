package ast

import (
	"testing"

	"github.com/birdie-ai/katschema/parser/token"
	"github.com/google/go-cmp/cmp"
)

func TestTupleStorage(t *testing.T) {
	t.Parallel()
	tree := New()
	span := token.UnknownSpan()
	one := tree.AddInt("1", span)
	list := tree.AddList([]NodeID{one}, span)
	tuple := tree.AddTuple([]NodeID{list, one}, span)
	if diff := cmp.Diff([]NodeID{list, one}, tree.Tuple(tuple)); diff != "" {
		t.Fatalf("tuple elements differ (-want +got):\n%s", diff)
	}
	if tree.Tuple(list) != nil || tree.List(tuple) != nil {
		t.Fatal("list and tuple accessors must distinguish their node kinds")
	}
	if tree.Node(tuple).Span() != span {
		t.Fatalf("synthetic tuple span = %v, want %v", tree.Node(tuple).Span(), span)
	}
	tree.Reset()
	if tree.Node(tuple).Kind() != Invalid || tree.Tuple(tuple) != nil {
		t.Fatal("reset retained a tuple node")
	}
	empty := tree.AddTuple(nil, span)
	if tree.Node(empty).Kind() != Tuple || len(tree.Tuple(empty)) != 0 {
		t.Fatal("reused tree did not produce an empty tuple")
	}
}
