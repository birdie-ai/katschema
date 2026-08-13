package ast

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/birdie-ai/katschema/parser/token"
)

// Print writes root in compact Katschema syntax.
//
// Print preserves AST order.
func Print(w io.Writer, t *Tree, root NodeID) error {
	p := printer{w: w, t: t}
	p.node(root)
	return p.err
}

type printer struct {
	w   io.Writer
	t   *Tree
	err error
}

func (p *printer) node(id NodeID) {
	if p.err != nil {
		return
	}

	switch n := p.t.Node(id); n.Kind() {
	case Null:
		p.write("null")

	case Bool:
		if p.t.Bool(id) {
			p.write("true")
		} else {
			p.write("false")
		}

	case Int:
		p.write(p.t.Int(id))

	case Float:
		p.write(p.t.Float(id))

	case String:
		p.string(p.t.String(id))

	case List:
		p.write("[")
		for i, v := range p.t.Array(id) {
			if i != 0 {
				p.write(",")
			}
			p.node(v)
		}
		p.write("]")

	case Object:
		p.write("{")
		for i, f := range p.t.Object(id) {
			if i != 0 {
				p.write(",")
			}
			p.string(p.t.Text(f.Name))
			p.write(":")
			p.node(f.Value)
		}
		p.write("}")

	case Schema:
		s := p.t.Schema(id)
		p.write("(")
		p.node(s.Type)
		for _, c := range s.Clauses {
			p.write(",")
			p.node(c)
		}
		p.write(")")

	case Name:
		p.write(p.t.Name(id))

	case Path:
		for _, part := range p.t.Path(id) {
			p.write(".")
			p.write(p.t.Text(part.Name))
		}

	case Constraint:
		p.node(p.t.Constraint(id))

	case Attr:
		a := p.t.Attr(id)
		p.write(p.t.Text(a.Name))
		if a.HasValue {
			p.write("=")
			p.node(a.Value)
		}

	case Ident:
		p.write(p.t.Ident(id))

	case Call:
		c := p.t.Fncall(id)
		p.write(p.t.Text(c.Func))
		p.write("(")
		for i, a := range c.Args {
			if i != 0 {
				p.write(",")
			}
			p.node(a)
		}
		p.write(")")

	case Unary:
		u := p.t.Unary(id)
		if !u.Op.IsUnary() {
			p.failf("invalid unary operator %q", u.Op.String())
			return
		}
		p.write(u.Op.String())
		p.node(u.X)

	case Binary:
		b := p.t.Binary(id)
		if !b.Op.IsBinary() {
			p.failf("invalid binary operator %q", b.Op.String())
			return
		}
		p.node(b.Left)
		if b.Op == token.IN {
			p.write(" in ")
		} else {
			p.write(b.Op.String())
		}
		p.node(b.Right)

	case Group:
		p.write("(")
		p.node(p.t.Group(id))
		p.write(")")

	default:
		p.failf("invalid node %d", id)
	}
}

func (p *printer) string(s string) {
	if p.err != nil {
		return
	}
	b, err := json.Marshal(s)
	if err != nil {
		p.err = err
		return
	}
	_, p.err = p.w.Write(b)
}

func (p *printer) write(s string) {
	if p.err != nil {
		return
	}
	_, p.err = io.WriteString(p.w, s)
}

func (p *printer) failf(format string, args ...any) {
	if p.err == nil {
		p.err = fmt.Errorf("ast: "+format, args...)
	}
}
