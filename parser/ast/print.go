package ast

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/birdie-ai/katschema/parser/token"
)

// Print writes root in compact Katschema syntax.
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

	case Decimal:
		p.write(p.t.Decimal(id))

	case String:
		p.string(p.t.String(id))

	case List:
		p.write("[")
		p.elements(p.t.List(id))
		p.write("]")

	case Tuple:
		p.write("#{")
		p.elements(p.t.Tuple(id))
		p.write("}")

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
		if k := p.t.Node(s.Type).Kind(); k != Sum && !k.IsTypeRef() {
			panic(fmt.Sprintf("ast: invalid schema type %s at node %d", k, s.Type))
		}
		p.write("(")
		p.node(s.Type)
		for _, c := range s.Clauses {
			p.write(",")
			p.node(c)
		}
		p.write(")")

	case Sum:
		s := p.t.Sum(id)
		p.sumMember(s.Left)
		p.write("|")
		p.sumMember(s.Right)
	case Name:
		p.write(p.t.Name(id))

	case Path:
		for _, part := range p.t.Path(id) {
			p.write(".")
			p.write(p.t.Text(part.Name))
		}

	case Constraint:
		expr := p.t.Constraint(id)
		parens := p.t.Node(expr).Kind() == Ident
		if parens {
			p.write("(")
		}
		p.node(expr)
		if parens {
			p.write(")")
		}

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
		p.elements(c.Args)
		p.write(")")

	case Unary, Binary:
		p.expr(id, 0)

	case Group:
		p.write("(")
		p.node(p.t.Group(id))
		p.write(")")

	default:
		p.failf("invalid node %d", id)
	}
}

func (p *printer) sumMember(id NodeID) {
	if p.err != nil {
		return
	}
	if k := p.t.Node(id).Kind(); k != Sum && !k.IsValue() && !k.IsTypeRef() {
		panic(fmt.Sprintf("ast: invalid sum member %s at node %d", k, id))
	}
	p.node(id)
}

func (p *printer) elements(elems []NodeID) {
	for i, v := range elems {
		if i != 0 {
			p.write(",")
		}
		p.node(v)
	}
}

func (p *printer) expr(id NodeID, minPrec int) {
	if p.err != nil {
		return
	}
	switch p.t.Node(id).Kind() {
	case Unary:
		u := p.t.Unary(id)
		if !u.Op.IsUnary() {
			p.failf("invalid unary operator %q", u.Op.String())
			return
		}
		p.write(u.Op.String())
		kind := p.t.Node(u.X).Kind()
		if u.Op == token.Sub && (kind == Int || kind == Decimal) {
			p.write("(")
			p.node(u.X)
			p.write(")")
		} else {
			p.expr(u.X, 7)
		}
	case Binary:
		b := p.t.Binary(id)
		prec := b.Op.Precedence()
		if prec == 0 {
			p.failf("invalid binary operator %q", b.Op.String())
			return
		}
		parens := prec < minPrec
		if parens {
			p.write("(")
		}
		leftPrec := prec
		switch b.Op {
		case token.Eq, token.Ne, token.In, token.Match:
			leftPrec++
		}
		p.expr(b.Left, leftPrec)
		if b.Op == token.In {
			p.write(" in ")
		} else {
			p.write(b.Op.String())
		}
		p.expr(b.Right, prec+1)
		if parens {
			p.write(")")
		}
	default:
		p.node(id)
	}
}

func (p *printer) string(s string) {
	if p.err != nil {
		return
	}
	b, _ := json.Marshal(s)
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
