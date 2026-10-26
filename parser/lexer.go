package parser

import (
	"unicode"
	"unicode/utf8"

	"github.com/birdie-ai/katschema/parser/token"
)

type lexer struct {
	src string
	off int
}

type lexeme struct {
	kind token.Kind
	span token.Span
}

func (l *lexer) next() (lexeme, error) {
	if err := l.skipSpace(); err != nil {
		return lexeme{}, err
	}
	start := l.off
	if start == len(l.src) {
		return lexeme{kind: token.EOF, span: token.NewSpan(start, start)}, nil
	}

	c := l.src[l.off]
	var kind token.Kind
	switch {
	case letter(c) || c == '_':
		kind = l.scanIdent()
	case digit(c):
		var err error
		kind, err = l.scanNumber()
		if err != nil {
			return lexeme{}, err
		}
	case c == '"':
		if err := l.scanString(); err != nil {
			return lexeme{}, err
		}
		kind = token.String
	case c >= utf8.RuneSelf:
		r, _ := utf8.DecodeRuneInString(l.src[l.off:])
		if !unicode.IsLetter(r) {
			return lexeme{}, l.errorAt(start, "unexpected character")
		}
		kind = l.scanIdent()
	default:
		l.off++
		switch c {
		case '(':
			kind = token.LParen
		case ')':
			kind = token.RParen
		case '[':
			kind = token.LBrack
		case ']':
			kind = token.RBrack
		case '{':
			kind = token.LBrace
		case '}':
			kind = token.RBrace
		case ',':
			kind = token.Comma
		case ':':
			kind = token.Colon
		case '.':
			kind = token.Dot
		case '=':
			kind = token.Assign
			if l.accept('=') {
				kind = token.Eq
			}
		case '!':
			kind = token.Not
			if l.accept('=') {
				kind = token.Ne
			}
		case '<':
			kind = token.Lt
			if l.accept('=') {
				kind = token.Le
			}
		case '>':
			kind = token.Gt
			if l.accept('=') {
				kind = token.Ge
			}
		case '~':
			kind = token.Match
		case '&':
			if !l.accept('&') {
				return lexeme{}, l.errorAt(start, "expected &&")
			}
			kind = token.And
		case '|':
			kind = token.Pipe
			if l.accept('|') {
				kind = token.Or
			}
		case '+':
			kind = token.Add
		case '-':
			kind = token.Sub
		case '*':
			kind = token.Mul
		case '/':
			kind = token.Div
			if l.accept('/') {
				kind = token.Comment
				for l.off < len(l.src) && l.src[l.off] != '\n' {
					l.off++
				}
			} else if l.accept('*') {
				kind = token.Comment
				for l.off < len(l.src) && !(l.src[l.off] == '*' && l.off+1 < len(l.src) && l.src[l.off+1] == '/') {
					l.off++
				}
				if l.off == len(l.src) {
					return lexeme{}, syntaxError(start, start+2, "unterminated block comment")
				}
				l.off += 2
			}
		case '%':
			kind = token.Mod
		case '#':
			if !l.accept('{') {
				return lexeme{}, l.errorAt(start, "expected #{")
			}
			kind = token.TupleOpen
		default:
			return lexeme{}, l.errorAt(start, "unexpected character")
		}
	}
	return lexeme{kind: kind, span: token.NewSpan(start, l.off)}, nil
}

func (l *lexer) skipSpace() error {
	for l.off < len(l.src) {
		c := l.src[l.off]
		switch {
		case c == ' ' || c >= '\t' && c <= '\r':
			l.off++
		case c >= utf8.RuneSelf:
			r, size := utf8.DecodeRuneInString(l.src[l.off:])
			if r == utf8.RuneError && size == 1 {
				return l.errorAt(l.off, "invalid UTF-8")
			}
			if !unicode.IsSpace(r) {
				return nil
			}
			l.off += size
		default:
			return nil
		}
	}
	return nil
}

func (l *lexer) scanIdent() token.Kind {
	// NOTE(i4k): For simplicity this method parses an invalid ident (eg.: 1test).
	// The caller must ensure to call it only in valid identStart (check grammer.ohm).
	start := l.off
	for l.off < len(l.src) {
		c := l.src[l.off]
		if c < utf8.RuneSelf {
			if !letter(c) && !digit(c) && c != '_' {
				break
			}
			l.off++
			continue
		}
		r, size := utf8.DecodeRuneInString(l.src[l.off:])
		if !unicode.IsLetter(r) {
			break
		}
		l.off += size
	}
	switch l.src[start:l.off] {
	case "null":
		return token.Null
	case "true":
		return token.True
	case "false":
		return token.False
	case "in":
		return token.In
	}
	return token.Ident
}

func (l *lexer) scanNumber() (token.Kind, error) {
	if l.accept('0') {
		if l.off < len(l.src) && digit(l.src[l.off]) {
			return token.Illegal, l.errorAt(l.off, "leading zero in number")
		}
	} else {
		for l.off < len(l.src) && digit(l.src[l.off]) {
			l.off++
		}
	}

	kind := token.Int
	if l.accept('.') {
		kind = token.Decimal
		if l.off == len(l.src) || !digit(l.src[l.off]) {
			return token.Illegal, l.errorAt(l.off, "expected digit after decimal point")
		}
		for l.off < len(l.src) && digit(l.src[l.off]) {
			l.off++
		}
	}
	if l.accept('e') || l.accept('E') {
		kind = token.Decimal
		if !l.accept('+') {
			l.accept('-')
		}
		if l.off == len(l.src) || !digit(l.src[l.off]) {
			return token.Illegal, l.errorAt(l.off, "expected exponent digit")
		}
		for l.off < len(l.src) && digit(l.src[l.off]) {
			l.off++
		}
	}
	return kind, nil
}

func (l *lexer) scanString() error {
	start := l.off
	l.off++
	for l.off < len(l.src) {
		switch c := l.src[l.off]; {
		case c == '"':
			l.off++
			return nil
		case c == '\\':
			escape := l.off
			l.off++
			if l.off == len(l.src) {
				return syntaxError(escape, escape+1, "unterminated escape")
			}
			switch l.src[l.off] {
			case '"', '\\', '/', 'b', 'f', 'n', 'r', 't':
				l.off++
			case 'u':
				l.off++
				for i := 0; i < 4; i++ {
					if l.off == len(l.src) || !hexDigit(l.src[l.off]) {
						return l.errorAt(l.off, "expected hexadecimal digit")
					}
					l.off++
				}
			default:
				return l.errorAt(l.off, "invalid string escape")
			}
		case c < 0x20:
			return l.errorAt(l.off, "unescaped control character in string")
		default:
			l.off++
		}
	}
	return syntaxError(start, start+1, "unterminated string")
}

func (l *lexer) accept(c byte) bool {
	if l.off == len(l.src) || l.src[l.off] != c {
		return false
	}
	l.off++
	return true
}

func (l *lexer) errorAt(off int, message string) error {
	end := off
	if off < len(l.src) {
		_, size := utf8.DecodeRuneInString(l.src[off:])
		end += size
	}
	return syntaxError(off, end, message)
}

func letter(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

func digit(c byte) bool {
	return c >= '0' && c <= '9'
}

func hexDigit(c byte) bool {
	return digit(c) || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}
