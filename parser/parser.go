package parser

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/birdie-ai/katschema/parser/ast"
	"github.com/birdie-ai/katschema/parser/token"
)

// same limit of encoding/json:
// https://github.com/golang/go/blob/0d37007fda071631fbcb520b99c85a8e4c93108a/src/encoding/json/scanner.go#L148
const maxDepth = 10000

type parser struct {
	lex   lexer
	tok   lexeme
	tree  *ast.Tree
	depth int
}

// Parse one root value from src.
func Parse(src string) (*ast.Tree, ast.NodeID, error) {
	p := parser{lex: lexer{src: src}, tree: ast.New()}
	if err := p.next(); err != nil {
		return nil, 0, err
	}
	root, err := p.value()
	if err != nil {
		return nil, 0, err
	}
	if p.tok.kind != token.EOF {
		return nil, 0, p.expected("EOF")
	}
	return p.tree, root, nil
}

func (p *parser) read() (lexeme, error) {
	for {
		x, err := p.lex.next()
		if err != nil {
			return lexeme{}, err
		}
		if x.kind != token.Comment {
			return x, nil
		}
		p.tree.AddComment(p.text(x), x.span)
	}
}

func (p *parser) next() error {
	x, err := p.read()
	if err != nil {
		return err
	}
	p.tok = x
	return nil
}

func (p *parser) accept(kind token.Kind) (bool, error) {
	if p.tok.kind != kind {
		return false, nil
	}
	return true, p.next()
}

func (p *parser) expect(kind token.Kind) (lexeme, error) {
	if p.tok.kind != kind {
		return lexeme{}, p.expected(kind.String())
	}
	x := p.tok
	return x, p.next()
}

func (p *parser) expected(want string) error {
	return &Error{Span: p.tok.span, Message: fmt.Sprintf("expected %s, found %s", want, p.tok.kind)}
}

func (p *parser) text(x lexeme) string {
	return p.lex.src[x.span.Start:x.span.End]
}

func (p *parser) enter() error {
	if p.depth == maxDepth {
		return &Error{Span: p.tok.span, Message: "maximum nesting depth exceeded"}
	}
	p.depth++
	return nil
}

func (p *parser) value() (ast.NodeID, error) {
	if p.tok.kind == token.LParen {
		return p.schema()
	}
	return p.jsonValue()
}

func (p *parser) jsonValue() (ast.NodeID, error) {
	x := p.tok
	switch x.kind {
	case token.LBrace:
		return p.object()
	case token.LBrack:
		return p.list()
	case token.Sub, token.Int, token.Decimal:
		return p.number()
	case token.String:
		value, err := p.stringLiteral()
		if err != nil {
			return 0, err
		}
		return p.tree.AddString(value, x.span), nil
	case token.Null:
		if err := p.next(); err != nil {
			return 0, err
		}
		return p.tree.AddNull(x.span), nil
	case token.True, token.False:
		if err := p.next(); err != nil {
			return 0, err
		}
		return p.tree.AddBool(x.kind == token.True, x.span), nil
	default:
		return 0, p.expected("value")
	}
}

func (p *parser) number() (ast.NodeID, error) {
	start := p.tok.span.Start
	if p.tok.kind == token.Sub {
		minus := p.tok
		if err := p.next(); err != nil {
			return 0, err
		}
		if p.tok.kind != token.Int && p.tok.kind != token.Decimal {
			return 0, p.expected("number")
		}
		if p.tok.span.Start != minus.span.End {
			return 0, &Error{Span: p.tok.span, Message: "number must immediately follow -"}
		}
	}
	x := p.tok
	span := token.Span{Start: start, End: x.span.End}
	raw := p.lex.src[span.Start:span.End]
	if err := p.next(); err != nil {
		return 0, err
	}
	if x.kind == token.Int {
		return p.tree.AddInt(raw, span), nil
	}
	return p.tree.AddDecimal(raw, span), nil
}

func (p *parser) stringLiteral() (string, error) {
	if p.tok.kind != token.String {
		return "", p.expected("string")
	}
	var value string
	if err := json.Unmarshal([]byte(p.text(p.tok)), &value); err != nil {
		return "", &Error{Span: p.tok.span, Message: err.Error()}
	}
	return value, p.next()
}

func (p *parser) object() (ast.NodeID, error) {
	if err := p.enter(); err != nil {
		return 0, err
	}
	defer func() { p.depth-- }()
	open, err := p.expect(token.LBrace)
	if err != nil {
		return 0, err
	}
	var fields []ast.Field
	for p.tok.kind != token.RBrace {
		key := p.tok
		name, err := p.stringLiteral()
		if err != nil {
			return 0, err
		}
		if _, err := p.expect(token.Colon); err != nil {
			return 0, err
		}
		value, err := p.value()
		if err != nil {
			return 0, err
		}
		span := token.Span{Start: key.span.Start, End: p.tree.Node(value).Span().End}
		fields = append(fields, p.tree.NewField(name, value, span, key.span))
		comma, err := p.accept(token.Comma)
		if err != nil {
			return 0, err
		}
		if !comma {
			break
		}
	}
	close, err := p.expect(token.RBrace)
	if err != nil {
		return 0, err
	}
	return p.tree.AddObject(fields, token.Span{Start: open.span.Start, End: close.span.End}), nil
}

func (p *parser) list() (ast.NodeID, error) {
	if err := p.enter(); err != nil {
		return 0, err
	}
	defer func() { p.depth-- }()
	open, err := p.expect(token.LBrack)
	if err != nil {
		return 0, err
	}
	var elems []ast.NodeID
	for p.tok.kind != token.RBrack {
		value, err := p.value()
		if err != nil {
			return 0, err
		}
		elems = append(elems, value)
		comma, err := p.accept(token.Comma)
		if err != nil {
			return 0, err
		}
		if !comma {
			break
		}
	}
	close, err := p.expect(token.RBrack)
	if err != nil {
		return 0, err
	}
	return p.tree.AddList(elems, token.Span{Start: open.span.Start, End: close.span.End}), nil
}

func (p *parser) schema() (ast.NodeID, error) {
	if err := p.enter(); err != nil {
		return 0, err
	}
	defer func() { p.depth-- }()
	open, err := p.expect(token.LParen)
	if err != nil {
		return 0, err
	}
	typ, err := p.typeAtom()
	if err != nil {
		return 0, err
	}
	var clauses []ast.NodeID
	for {
		comma, err := p.accept(token.Comma)
		if err != nil {
			return 0, err
		}
		if !comma || p.tok.kind == token.RParen {
			break
		}
		clause, err := p.clause()
		if err != nil {
			return 0, err
		}
		clauses = append(clauses, clause)
	}
	close, err := p.expect(token.RParen)
	if err != nil {
		return 0, err
	}
	return p.tree.AddSchema(typ, clauses, token.Span{Start: open.span.Start, End: close.span.End}), nil
}

func (p *parser) typeAtom() (ast.NodeID, error) {
	switch p.tok.kind {
	case token.Ident, token.Dot:
		return p.ref()
	case token.LBrace:
		return p.object()
	case token.LBrack:
		return p.list()
	default:
		return 0, p.expected("type reference")
	}
}

func (p *parser) ref() (ast.NodeID, error) {
	start := p.tok.span.Start
	path, err := p.accept(token.Dot)
	if err != nil {
		return 0, err
	}
	var names []string
	var parts []ast.PathPart
	var end token.Pos
	for {
		switch p.tok.kind {
		case token.Ident, token.True, token.False, token.Null, token.In:
		default:
			return 0, p.expected("reference component")
		}
		x := p.tok
		if path {
			parts = append(parts, p.tree.NewPathPart(p.text(x), x.span))
		} else {
			names = append(names, p.text(x))
		}
		end = x.span.End
		if err := p.next(); err != nil {
			return 0, err
		}
		dot, err := p.accept(token.Dot)
		if err != nil {
			return 0, err
		}
		if !dot {
			break
		}
	}
	span := token.Span{Start: start, End: end}
	if path {
		return p.tree.AddPath(parts, span), nil
	}
	return p.tree.AddName(strings.Join(names, "."), span), nil
}

func (p *parser) clause() (ast.NodeID, error) {
	name, err := p.expect(token.Ident)
	if err != nil {
		return 0, err
	}
	assign, err := p.accept(token.Assign)
	if err != nil {
		return 0, err
	}
	if !assign {
		return p.tree.AddAttr(p.text(name), 0, false, name.span, name.span), nil
	}
	value, err := p.jsonValue()
	if err != nil {
		return 0, err
	}
	span := token.Span{Start: name.span.Start, End: p.tree.Node(value).Span().End}
	return p.tree.AddAttr(p.text(name), value, true, span, name.span), nil
}
