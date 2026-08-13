// Package token defines lexical tokens and source positions used by the parser.
package token

// Pos is a byte position plus one. Zero means no position.
type Pos uint32

const NoPos Pos = 0

// At converts a zero-based byte offset to a Pos.
func At(off int) Pos {
	if off < 0 {
		return NoPos
	}
	return Pos(off) + 1
}

// Offset returns the zero-based byte offset for p, or -1 for NoPos.
func (p Pos) Offset() int {
	if p == NoPos {
		return -1
	}
	return int(p - 1)
}

// Span is a half-open source range [Start, End).
// A zero Span is unknown, as is common for synthetic AST nodes.
type Span struct {
	Start Pos
	End   Pos
}

func NewSpan(start, end int) Span {
	return Span{Start: At(start), End: At(end)}
}

func (s Span) Valid() bool {
	return s.Start != NoPos && s.End != NoPos && s.Start <= s.End
}

// Kind is a lexical token.
type Kind uint8

const (
	ILLEGAL Kind = iota
	EOF

	IDENT
	NUMBER
	STRING

	NULL
	TRUE
	FALSE
	IN

	LPAREN
	RPAREN
	LBRACK
	RBRACK
	LBRACE
	RBRACE
	COMMA
	COLON
	DOT

	ASSIGN // =
	EQ     // ==
	NE     // !=
	LT     // <
	LE     // <=
	GT     // >
	GE     // >=
	MATCH  // ~

	AND // &&
	OR  // ||
	NOT // !

	ADD // +
	SUB // -
	MUL // *
	QUO // /
	REM // %
)

func (k Kind) String() string {
	switch k {
	case EOF:
		return "EOF"
	case IDENT:
		return "identifier"
	case NUMBER:
		return "number"
	case STRING:
		return "string"
	case NULL:
		return "null"
	case TRUE:
		return "true"
	case FALSE:
		return "false"
	case IN:
		return "in"
	case LPAREN:
		return "("
	case RPAREN:
		return ")"
	case LBRACK:
		return "["
	case RBRACK:
		return "]"
	case LBRACE:
		return "{"
	case RBRACE:
		return "}"
	case COMMA:
		return ","
	case COLON:
		return ":"
	case DOT:
		return "."
	case ASSIGN:
		return "="
	case EQ:
		return "=="
	case NE:
		return "!="
	case LT:
		return "<"
	case LE:
		return "<="
	case GT:
		return ">"
	case GE:
		return ">="
	case MATCH:
		return "~"
	case AND:
		return "&&"
	case OR:
		return "||"
	case NOT:
		return "!"
	case ADD:
		return "+"
	case SUB:
		return "-"
	case MUL:
		return "*"
	case QUO:
		return "/"
	case REM:
		return "%"
	}
	return "ILLEGAL"
}

func (k Kind) IsUnary() bool {
	return k == NOT || k == ADD || k == SUB
}

func (k Kind) IsBinary() bool {
	switch k {
	case EQ, NE, LT, LE, GT, GE, IN, MATCH,
		AND, OR, ADD, SUB, MUL, QUO, REM:
		return true
	}
	return false
}
