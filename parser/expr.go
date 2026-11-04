package parser

import (
	"github.com/birdie-ai/katschema/parser/ast"
	"github.com/birdie-ai/katschema/parser/token"
)

// expr parsing using "precedence climbing" algorithm.
// Details: https://eli.thegreenplace.net/2012/08/02/parsing-expressions-by-precedence-climbing
// NOTE: we (atm) only have left associativy in katschema.
func (p *parser) expr(minPrec int) (ast.NodeID, error) {
	left, err := p.unary()
	if err != nil {
		return 0, err
	}
	for {
		op := p.tok.kind
		prec := op.Precedence()
		if prec < minPrec {
			return left, nil
		}
		if err := p.next(); err != nil {
			return 0, err
		}
		right, err := p.expr(prec + 1)
		if err != nil {
			return 0, err
		}
		span := token.Span{Start: p.tree.Node(left).Span().Start, End: p.tree.Node(right).Span().End}
		left = p.tree.AddBinary(left, op, right, span)
		if prec == token.Eq.Precedence() && p.tok.kind.Precedence() == prec {
			return 0, p.expected("parenthesized comparison")
		}
	}
}

func (p *parser) unary() (ast.NodeID, error) {
	op := p.tok
	if !op.kind.IsUnary() {
		return p.primary()
	}
	if err := p.enter(); err != nil {
		return 0, err
	}
	defer func() { p.depth-- }()
	if err := p.next(); err != nil {
		return 0, err
	}
	if op.kind == token.Sub && (p.tok.kind == token.Int || p.tok.kind == token.Decimal) {
		x := p.tok
		raw := "-" + p.text(x)
		span := token.Span{Start: op.span.Start, End: x.span.End}
		if err := p.next(); err != nil {
			return 0, err
		}
		if x.kind == token.Int {
			return p.tree.AddInt(raw, span), nil
		}
		return p.tree.AddDecimal(raw, span), nil
	}
	x, err := p.unary()
	if err != nil {
		return 0, err
	}
	span := token.Span{Start: op.span.Start, End: p.tree.Node(x).Span().End}
	return p.tree.AddUnary(op.kind, x, span), nil
}

func (p *parser) primary() (ast.NodeID, error) {
	switch p.tok.kind {
	case token.Ident, token.Dot:
		return p.ref(ast.Ident)
	case token.LParen:
		return p.group()
	case token.Null, token.True, token.False, token.Int, token.Decimal, token.String, token.LBrace, token.LBrack, token.LTuple:
		return p.jsonValue()
	default:
		return 0, p.expected("expression")
	}
}

func (p *parser) group() (ast.NodeID, error) {
	if err := p.enter(); err != nil {
		return 0, err
	}
	defer func() { p.depth-- }()
	open, err := p.expect(token.LParen)
	if err != nil {
		return 0, err
	}
	x, err := p.expr(1)
	if err != nil {
		return 0, err
	}
	close, err := p.expect(token.RParen)
	if err != nil {
		return 0, err
	}
	return p.tree.AddGroup(token.Span{Start: open.span.Start, End: close.span.End}, x), nil
}

func (p *parser) call(name string, nameSpan token.Span) (ast.NodeID, error) {
	if err := p.enter(); err != nil {
		return 0, err
	}
	defer func() { p.depth-- }()
	if _, err := p.expect(token.LParen); err != nil {
		return 0, err
	}
	var args []ast.NodeID
	for p.tok.kind != token.RParen {
		x, err := p.expr(1)
		if err != nil {
			return 0, err
		}
		args = append(args, x)
		comma, err := p.accept(token.Comma)
		if err != nil {
			return 0, err
		}
		if !comma {
			break
		}
	}
	close, err := p.expect(token.RParen)
	if err != nil {
		return 0, err
	}
	span := token.Span{Start: nameSpan.Start, End: close.span.End}
	return p.tree.AddCall(name, args, span, nameSpan), nil
}
