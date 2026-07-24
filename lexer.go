package katschema

import (
	"errors"
	"fmt"
	"unicode/utf8"
)

// lexer does the lexical scan of a katschema text input.
// It works like an old-school tokenizer, where lexemes are not interpreted
// and produce only spans for each token kind.
// The eof is not treated as an error, then if Next() returns an error it means
// the lexer found something unexpected.
// When the end of the stream is reached it returns nil and lex.Token()
// returns an eof token. The caller must stop iterating once an eof token is returned.
// Why this? Because otherwise each error must be checked if it's an EOF error and
// treating EOF as an error is kinda odd as every stream eventually finishes so it's
// correct code path.
// Usage:
//
//	lex := newLexer(input)
//	for {
//		err := lex.Next()
//		if err != nil {
//			return err
//		}
//		tok := lex.Token() // returns the token value
//		lex.Text()  // returns the token text span.
//		lex.Bytes() // returns the token byte soan
//		if tok.kind == eof {
//			break
//		}
//	}
type lexer struct {
	in  []byte
	pos int

	// current state
	tok tokval
}

// lexical errors
var (
	errLexUnexpected = errors.New(`syntax error: unexpected character`)
	errLexNumber     = errors.New(`syntax error: invalid number`)
)

func newlexer(in []byte) *lexer {
	return &lexer{
		in: in,
	}
}

func (l *lexer) Next() error {
	l.skipws()
	if l.eof() {
		return nil
	}

	// scan of each top-level entrypoint symbol.
	switch l.peek() {
	case '/':
		return l.scanSlash()
	case 'n':
		return l.scanNull()
	case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return l.scanNumber()
	default:
		return l.unexpected()
	}
}

func (l *lexer) Token() tokval {
	return l.tok
}

func (l *lexer) Text() string {
	return string(l.in[l.tok.start:l.tok.end])
}

func (l *lexer) peek() byte {
	return l.in[l.pos]
}

func (l *lexer) can() bool {
	return l.pos < len(l.in)
}

func (l *lexer) eat(n int) { l.pos += n }

func (l *lexer) eof() bool {
	if l.pos >= len(l.in) {
		l.tok = eoftok()
		return true
	}
	return false
}

func (l *lexer) unexpected() error {
	r, _ := utf8.DecodeRune(l.in[l.pos:])
	return fmt.Errorf("%w: %q at byte %d", errLexUnexpected, r, l.pos)
}

func (l *lexer) errorf(format string, args ...any) error {
	return fmt.Errorf(format, args...)
}

func (l *lexer) scanNull() error {
	start := l.pos
	l.eat(1)
	for _, b := range []byte{'u', 'l', 'l'} {
		if l.peek() != b {
			return l.unexpected()
		}
		l.eat(1)
	}
	l.tok = tokval{
		kind:  nul,
		start: start,
		end:   l.pos,
	}
	return nil
}

func (l *lexer) scanNumber() error {
	start := l.pos
	c := l.peek()
	if c == '-' {
		l.eat(1)
		if !l.can() {
			return l.errorf("%w: expected a digit following '-'", errLexNumber)
		}
		c = l.peek()
	}
	// integer
	switch c {
	case '0':
		l.eat(1)
	default:
		if !isdigit19(c) {
			return l.errorf("%w: expected 1-9 digit", errLexNumber)
		}
		l.eat(1)
		for l.can() && isdigit(l.peek()) {
			l.eat(1)
		}
	}

	// TODO(i4k): fraction part
	// TODO(i4k): exponent part
	l.tok = tokval{
		kind:  num,
		start: start,
		end:   l.pos,
	}
	return nil
}

func (l *lexer) scanSlash() error {
	l.eat(1)
	if l.peek() != '/' {
		return l.unexpected()
	}
	l.eat(1)
	start := l.pos
	end := start
	for l.can() && l.peek() != '\n' {
		l.eat(1)
	}
	end = l.pos
	l.tok = tokval{
		kind:  com,
		start: start,
		end:   end,
	}
	return nil
}

func (l *lexer) skipws() {
	for l.can() {
		switch l.peek() {
		case ' ', '\t', '\n', '\r':
			l.eat(1)
		default:
			return
		}
	}
}

func isdigit(b byte) bool {
	return '0' <= b && b <= '9'
}

func isdigit19(b byte) bool {
	return '1' <= b && b <= '9'
}
